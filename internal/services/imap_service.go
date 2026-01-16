// Package services 业务服务层
//
// imap_service.go IMAP邮件服务（用于Hotmail等个人账户）
package services

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/quotedprintable"
	"net"
	"outlook-mail-manager/internal/models"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 预编译正则表达式（性能优化）
var (
	existsRe    = regexp.MustCompile(`\* (\d+) EXISTS`)
	msgRe       = regexp.MustCompile(`MESSAGES\s+(\d+)`)
	unseenRe    = regexp.MustCompile(`UNSEEN\s+(\d+)`)
	uidRe       = regexp.MustCompile(`\* \d+ FETCH \(UID (\d+)`)
	fromRe      = regexp.MustCompile(`(?m)^From:\s*(.+)`)
	toRe        = regexp.MustCompile(`(?m)^To:\s*(.+)`)
	subjRe      = regexp.MustCompile(`(?m)^Subject:\s*(.+)`)
	dateRe      = regexp.MustCompile(`(?m)^Date:\s*(.+)`)
	seenRe      = regexp.MustCompile(`FLAGS \([^)]*\\Seen[^)]*\)`)
	boundaryRe  = regexp.MustCompile(`boundary="?([^"\s\r\n]+)"?`)
	htmlTagRe   = regexp.MustCompile(`<[^>]*>`)
)

// IMAPService IMAP邮件服务
type IMAPService struct {
	pool   map[string]*pooledClient // 连接池: email -> client
	poolMu sync.RWMutex
}

// pooledClient 池化的IMAP客户端
type pooledClient struct {
	client    *IMAPClient
	email     string
	token     string
	lastUsed  time.Time
	createdAt time.Time
}

// IMAP文件夹名映射（REST API ID -> IMAP名称）
var imapFolderMap = map[string]string{
	"inbox":        "INBOX",
	"junkemail":    "Junk",
	"drafts":       "Drafts",
	"sentitems":    "Sent",
	"deleteditems": "Deleted",
	"outbox":       "Outbox",
	"notes":        "Notes",
	"archive":      "Archive",
}

// reverseImapFolderMap 反向映射（IMAP名称 -> REST API ID）
// 包含英文名和UTF-7编码名
var reverseImapFolderMap = map[string]string{
	"INBOX":            "inbox",
	"Junk":             "junkemail",
	"Junk E-mail":      "junkemail",
	"Junk E-Mail":      "junkemail",
	"&Xn9USpCuTvY-":    "junkemail", // 垃圾邮件 UTF-7
	"Drafts":           "drafts",
	"&g0l6P3ux-":       "drafts", // 草稿 UTF-7
	"Sent":             "sentitems",
	"Sent Items":       "sentitems",
	"&UXZO1mWHTvZZOQ-": "sentitems", // 已发送邮件 UTF-7
	"Deleted":          "deleteditems",
	"Deleted Items":    "deleteditems",
	"&V4NXPpCuTvY-":    "deleteditems", // 已删除邮件 UTF-7
	"Outbox":           "outbox",
	"Notes":            "notes",
	"Archive":          "archive",
	"&W1hoYw-":         "archive", // 存档 UTF-7
}

// folderDisplayNames 文件夹ID到中文显示名的映射
var folderDisplayNames = map[string]string{
	"inbox":        "收件箱",
	"junkemail":    "垃圾邮件",
	"drafts":       "草稿",
	"sentitems":    "已发送",
	"deleteditems": "已删除",
	"outbox":       "发件箱",
	"notes":        "便笺",
	"archive":      "存档",
}

// MapFolderID 将REST API文件夹ID映射为IMAP文件夹名
func MapFolderID(folderID string) string {
	lower := strings.ToLower(folderID)
	if mapped, ok := imapFolderMap[lower]; ok {
		return mapped
	}
	return folderID
}

// getRestAPIFolderID 将IMAP文件夹名映射为REST API ID（大小写不敏感）
func getRestAPIFolderID(imapName string) string {
	// 精确匹配
	if id, ok := reverseImapFolderMap[imapName]; ok {
		return id
	}
	// 大小写不敏感匹配
	lowerName := strings.ToLower(imapName)
	for k, v := range reverseImapFolderMap {
		if strings.ToLower(k) == lowerName {
			return v
		}
	}
	return strings.ToLower(imapName)
}

// NewIMAPService 创建IMAPService实例
func NewIMAPService() *IMAPService {
	return &IMAPService{
		pool: make(map[string]*pooledClient),
	}
}

// getClient 从连接池获取或创建新连接
func (s *IMAPService) getClient(email, accessToken string) (*IMAPClient, error) {
	s.poolMu.Lock()
	defer s.poolMu.Unlock()

	// 检查是否有可用的缓存连接
	if pc, ok := s.pool[email]; ok {
		age := time.Since(pc.lastUsed)
		tokenMatch := pc.token == accessToken
		log.Printf("[IMAP Pool] 找到缓存连接 - email: %s, age: %v, tokenMatch: %v", email, age, tokenMatch)
		// 连接有效期5分钟，token必须相同
		if age < 5*time.Minute && tokenMatch {
			pc.lastUsed = time.Now()
			log.Printf("[IMAP Pool] 复用缓存连接 - email: %s", email)
			return pc.client, nil
		}
		// 连接过期或token变化，关闭旧连接
		log.Printf("[IMAP Pool] 连接过期或token变化，关闭旧连接 - email: %s, age: %v, tokenMatch: %v", email, age, tokenMatch)
		pc.client.Close()
		delete(s.pool, email)
	} else {
		log.Printf("[IMAP Pool] 无缓存连接 - email: %s", email)
	}

	// 创建新连接
	log.Printf("[IMAP Pool] 创建新连接 - email: %s", email)
	client, err := newIMAPClient(email, accessToken)
	if err != nil {
		log.Printf("[IMAP Pool] 创建连接失败 - email: %s, error: %v", email, err)
		return nil, err
	}
	log.Printf("[IMAP Pool] 创建连接成功 - email: %s", email)

	// 缓存连接
	s.pool[email] = &pooledClient{
		client:    client,
		email:     email,
		token:     accessToken,
		lastUsed:  time.Now(),
		createdAt: time.Now(),
	}
	log.Printf("[IMAP Pool] 连接已缓存 - email: %s, 池大小: %d", email, len(s.pool))

	return client, nil
}

// IMAPClient 简单的IMAP客户端
type IMAPClient struct {
	conn   net.Conn
	buffer []byte
	tagNum int
}

// getIMAPServer 根据邮箱域名选择IMAP服务器
func getIMAPServer(email string) string {
	// 个人账户域名使用 imap-mail.outlook.com
	personalDomains := []string{"@hotmail.", "@outlook.", "@live.", "@msn."}
	emailLower := strings.ToLower(email)
	for _, domain := range personalDomains {
		if strings.Contains(emailLower, domain) {
			return "imap-mail.outlook.com:993"
		}
	}
	// 企业账户使用 outlook.office365.com
	return "outlook.office365.com:993"
}

// newIMAPClient 创建IMAP连接
func newIMAPClient(email, accessToken string) (*IMAPClient, error) {
	server := getIMAPServer(email)
	host := strings.Split(server, ":")[0]
	log.Printf("[IMAP Connect] 开始连接 %s - email: %s", server, email)

	// 使用tls.DialWithDialer，自动处理SNI和超时
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 30 * time.Second},
		"tcp",
		server,
		&tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		},
	)
	if err != nil {
		log.Printf("[IMAP Connect] TLS连接失败: %v", err)
		return nil, fmt.Errorf("connect failed: %w", err)
	}
	log.Printf("[IMAP Connect] TLS连接成功")

	client := &IMAPClient{conn: conn, buffer: make([]byte, 65536), tagNum: 0}

	// 读取欢迎消息
	log.Printf("[IMAP Connect] 读取欢迎消息...")
	welcome, err := client.readResponse()
	if err != nil {
		log.Printf("[IMAP Connect] 读取欢迎消息失败: %v", err)
		conn.Close()
		return nil, err
	}
	log.Printf("[IMAP Connect] 欢迎消息: %s", welcome)

	// XOAUTH2认证
	authStr := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", email, accessToken)
	authB64 := base64.StdEncoding.EncodeToString([]byte(authStr))
	log.Printf("[IMAP Connect] 发送 AUTHENTICATE XOAUTH2 命令...")

	authResp, err := client.command("AUTHENTICATE XOAUTH2 " + authB64)
	if err != nil {
		log.Printf("[IMAP Connect] 认证失败: %v", err)
		conn.Close()
		return nil, fmt.Errorf("auth failed: %w", err)
	}
	log.Printf("[IMAP Connect] 认证成功: %s", authResp)

	return client, nil
}

// command 发送IMAP命令并等待响应
//
// IMAP协议要求每个命令都带有唯一的标签（tag），服务器响应时会包含相同的标签。
// 本方法自动生成递增的标签（A001, A002, ...），发送命令并等待带有该标签的响应。
//
// 参数：
//   - cmd: IMAP命令字符串（不含标签），如 "LIST \"\" \"*\"" 或 "SELECT INBOX"
//
// 返回值：
//   - string: 服务器的完整响应内容
//   - error: 发送失败或服务器返回错误时的错误信息
//
// IMAP命令格式：
//
//	客户端: A001 SELECT INBOX\r\n
//	服务器: * 172 EXISTS\r\n
//	        * 1 RECENT\r\n
//	        A001 OK SELECT completed\r\n
func (c *IMAPClient) command(cmd string) (string, error) {
	c.tagNum++
	tag := fmt.Sprintf("A%03d", c.tagNum)
	_, err := c.conn.Write([]byte(tag + " " + cmd + "\r\n"))
	if err != nil {
		return "", err
	}
	return c.readUntilTag(tag)
}

// readResponse 读取服务器的单次响应
//
// 从IMAP连接读取一次数据到缓冲区。用于读取服务器的初始欢迎消息等
// 不需要等待特定标签的场景。
//
// 设置30秒读取超时，防止连接挂起。
//
// 返回值：
//   - string: 读取到的响应内容
//   - error: 读取超时或连接错误时返回错误
func (c *IMAPClient) readResponse() (string, error) {
	c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	n, err := c.conn.Read(c.buffer)
	if err != nil {
		return "", err
	}
	return string(c.buffer[:n]), nil
}

// readUntilTag 持续读取直到收到指定标签的响应
//
// IMAP服务器可能会发送多行响应（以*开头的非标签行），
// 本方法持续读取直到收到以指定标签开头的最终响应行。
//
// 响应状态判断：
//   - "tag OK": 命令执行成功
//   - "tag NO": 命令执行失败（如文件夹不存在）
//   - "tag BAD": 命令语法错误
//
// 参数：
//   - tag: 要等待的命令标签（如 "A001"）
//
// 返回值：
//   - string: 完整的响应内容（包括所有中间行）
//   - error: 超时、连接错误或服务器返回NO/BAD时返回错误
func (c *IMAPClient) readUntilTag(tag string) (string, error) {
	var result strings.Builder
	for {
		c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := c.conn.Read(c.buffer)
		if err != nil {
			return result.String(), err
		}
		result.Write(c.buffer[:n])
		resp := result.String()
		if strings.Contains(resp, tag+" OK") {
			return resp, nil
		}
		if strings.Contains(resp, tag+" NO") || strings.Contains(resp, tag+" BAD") {
			return resp, fmt.Errorf("IMAP error: %s", resp)
		}
	}
}

// Close 关闭IMAP连接
//
// 按照IMAP协议规范，先发送LOGOUT命令通知服务器断开连接，
// 然后关闭底层TCP连接。这是优雅关闭连接的标准方式。
//
// 注意：连接池管理的连接不应直接调用此方法，
// 而是由连接池在连接过期时自动调用。
func (c *IMAPClient) Close() {
	c.command("LOGOUT")
	c.conn.Close()
}

// GetMailFolders 获取邮件文件夹列表
func (s *IMAPService) GetMailFolders(email, accessToken string) ([]models.MailFolder, error) {
	log.Printf("[IMAP] GetMailFolders 开始 - email: %s", email)

	client, err := s.getClient(email, accessToken)
	if err != nil {
		log.Printf("[IMAP] getClient 失败: %v", err)
		return nil, err
	}
	log.Printf("[IMAP] getClient 成功，连接已建立")
	// 不关闭连接，保持在连接池中

	log.Printf("[IMAP] 发送 LIST 命令...")
	resp, err := client.command("LIST \"\" \"*\"")
	if err != nil {
		log.Printf("[IMAP] LIST 命令失败: %v", err)
		return nil, err
	}
	log.Printf("[IMAP] LIST 响应长度: %d 字节", len(resp))
	log.Printf("[IMAP] LIST 响应内容:\n%s", resp)

	var folders []models.MailFolder
	// 存储原始IMAP名称用于STATUS查询
	var imapNames []string
	lines := strings.Split(resp, "\r\n")
	log.Printf("[IMAP] 解析到 %d 行", len(lines))

	for _, line := range lines {
		if strings.HasPrefix(line, "* LIST") {
			log.Printf("[IMAP] 解析文件夹行: %s", line)
			// 解析: * LIST (\HasNoChildren) "/" Inbox 或 * LIST (\HasNoChildren) "/" "INBOX"
			// 找到最后一个 "/" 后面的文件夹名
			idx := strings.LastIndex(line, "\"/\" ")
			if idx == -1 {
				idx = strings.LastIndex(line, "\" ")
			}
			if idx != -1 {
				name := strings.TrimSpace(line[idx+4:])
				name = strings.Trim(name, "\"") // 移除可能的引号
				decoded := decodeIMAPUTF7(name)
				// 使用大小写不敏感的映射获取REST API风格的ID
				id := getRestAPIFolderID(name)
				// 使用中文显示名
				displayName := decoded
				if chineseName, ok := folderDisplayNames[id]; ok {
					displayName = chineseName
				}
				log.Printf("[IMAP] 文件夹: name=%s, decoded=%s, id=%s, displayName=%s", name, decoded, id, displayName)
				folders = append(folders, models.MailFolder{
					ID:          id,
					DisplayName: displayName,
				})
				imapNames = append(imapNames, name) // 保存原始名称
			}
		}
	}
	log.Printf("[IMAP] 共解析到 %d 个文件夹", len(folders))

	// 只获取收件箱和垃圾邮件的数量（前端只显示这两个）
	targetFolders := map[string]bool{"inbox": true, "junkemail": true}
	for i := range folders {
		if !targetFolders[folders[i].ID] {
			continue
		}
		log.Printf("[IMAP] 获取文件夹 %s (IMAP名: %s) 的计数...", folders[i].ID, imapNames[i])
		// 使用原始IMAP文件夹名进行STATUS查询
		statusCmd := fmt.Sprintf("STATUS \"%s\" (MESSAGES UNSEEN)", imapNames[i])
		log.Printf("[IMAP] 发送命令: %s", statusCmd)
		statusResp, err := client.command(statusCmd)
		if err != nil {
			log.Printf("[IMAP] STATUS 命令失败: %v", err)
			continue
		}
		log.Printf("[IMAP] STATUS 响应:\n%s", statusResp)

		// 解析: * STATUS "INBOX" (MESSAGES 10 UNSEEN 2)
		if m := msgRe.FindStringSubmatch(statusResp); len(m) > 1 {
			folders[i].TotalItemCount, _ = strconv.Atoi(m[1])
			log.Printf("[IMAP] 解析 MESSAGES: %s -> TotalItemCount=%d", m[1], folders[i].TotalItemCount)
		} else {
			log.Printf("[IMAP] 未能匹配 MESSAGES 正则: %s", msgRe.String())
		}
		if m := unseenRe.FindStringSubmatch(statusResp); len(m) > 1 {
			folders[i].UnreadItemCount, _ = strconv.Atoi(m[1])
			log.Printf("[IMAP] 解析 UNSEEN: %s -> UnreadItemCount=%d", m[1], folders[i].UnreadItemCount)
		} else {
			log.Printf("[IMAP] 未能匹配 UNSEEN 正则: %s", unseenRe.String())
		}
		log.Printf("[IMAP] 文件夹 %s 最终计数: Total=%d, Unread=%d", folders[i].ID, folders[i].TotalItemCount, folders[i].UnreadItemCount)
	}

	log.Printf("[IMAP] GetMailFolders 完成，返回 %d 个文件夹", len(folders))
	return folders, nil
}

// GetMessages 获取邮件列表
func (s *IMAPService) GetMessages(email, accessToken, folderID string, skip, top int) ([]models.Message, error) {
	log.Printf("[IMAP] GetMessages 开始 - email: %s, folderID: %s, skip: %d, top: %d", email, folderID, skip, top)

	client, err := s.getClient(email, accessToken)
	if err != nil {
		log.Printf("[IMAP] getClient 失败: %v", err)
		return nil, err
	}
	log.Printf("[IMAP] getClient 成功")
	// 不关闭连接，保持在连接池中

	// 映射文件夹名称
	imapFolder := MapFolderID(folderID)
	log.Printf("[IMAP] 文件夹映射: %s -> %s", folderID, imapFolder)

	// 选择文件夹
	selectCmd := fmt.Sprintf("SELECT \"%s\"", imapFolder)
	log.Printf("[IMAP] 发送命令: %s", selectCmd)
	selectResp, err := client.command(selectCmd)
	if err != nil {
		log.Printf("[IMAP] SELECT 命令失败: %v", err)
		return nil, err
	}
	log.Printf("[IMAP] SELECT 响应:\n%s", selectResp)

	// 解析邮件总数
	matches := existsRe.FindStringSubmatch(selectResp)
	log.Printf("[IMAP] EXISTS 正则匹配结果: %v", matches)
	if len(matches) < 2 {
		log.Printf("[IMAP] 未找到 EXISTS，返回空列表")
		return []models.Message{}, nil
	}
	total, _ := strconv.Atoi(matches[1])
	log.Printf("[IMAP] 邮件总数: %d", total)
	if total == 0 {
		log.Printf("[IMAP] 邮件总数为0，返回空列表")
		return []models.Message{}, nil
	}

	// 计算获取范围（从最新的开始）
	start := total - skip - top + 1
	end := total - skip
	log.Printf("[IMAP] 计算范围: total=%d, skip=%d, top=%d -> start=%d, end=%d", total, skip, top, start, end)
	if start < 1 {
		start = 1
		log.Printf("[IMAP] start 调整为 1")
	}
	if end < 1 {
		log.Printf("[IMAP] end < 1，返回空列表")
		return []models.Message{}, nil
	}

	// 获取邮件头
	fetchCmd := fmt.Sprintf("FETCH %d:%d (UID FLAGS BODY.PEEK[HEADER.FIELDS (FROM SUBJECT DATE)])", start, end)
	log.Printf("[IMAP] 发送命令: %s", fetchCmd)
	fetchResp, err := client.command(fetchCmd)
	if err != nil {
		log.Printf("[IMAP] FETCH 命令失败: %v", err)
		return nil, err
	}
	log.Printf("[IMAP] FETCH 响应长度: %d 字节", len(fetchResp))
	log.Printf("[IMAP] FETCH 响应内容:\n%s", fetchResp)

	messages := parseMessages(fetchResp)
	log.Printf("[IMAP] 解析到 %d 封邮件", len(messages))
	for i, msg := range messages {
		log.Printf("[IMAP] 邮件[%d]: ID=%s, Subject=%s, From=%v", i, msg.ID, msg.Subject, msg.From)
	}

	return messages, nil
}

// GetMessage 获取邮件详情
func (s *IMAPService) GetMessage(email, accessToken, folderID, messageID string) (*models.Message, error) {
	log.Printf("[IMAP] GetMessage 开始 - email: %s, folderID: %s, messageID: %s", email, folderID, messageID)

	client, err := s.getClient(email, accessToken)
	if err != nil {
		log.Printf("[IMAP] getClient 失败: %v", err)
		return nil, err
	}
	log.Printf("[IMAP] getClient 成功")
	// 不关闭连接，保持在连接池中

	// 映射文件夹名称
	imapFolder := MapFolderID(folderID)
	log.Printf("[IMAP] 文件夹映射: %s -> %s", folderID, imapFolder)

	selectCmd := fmt.Sprintf("SELECT \"%s\"", imapFolder)
	log.Printf("[IMAP] 发送命令: %s", selectCmd)
	selectResp, _ := client.command(selectCmd)
	log.Printf("[IMAP] SELECT 响应: %s", selectResp)

	// 使用UID获取完整邮件
	fetchCmd := fmt.Sprintf("UID FETCH %s (FLAGS BODY[])", messageID)
	log.Printf("[IMAP] 发送命令: %s", fetchCmd)
	fetchResp, err := client.command(fetchCmd)
	if err != nil {
		log.Printf("[IMAP] UID FETCH 失败: %v", err)
		return nil, err
	}
	log.Printf("[IMAP] UID FETCH 响应长度: %d 字节", len(fetchResp))

	msg := parseFullMessage(fetchResp)
	msg.ID = messageID // 设置邮件ID，用于前端匹配选中状态
	log.Printf("[IMAP] 解析邮件: ID=%s, Subject=%s, From=%v, BodyType=%s, BodyLen=%d",
		msg.ID, msg.Subject, msg.From, msg.Body.ContentType, len(msg.Body.Content))

	return msg, nil
}

// parseMessages 解析邮件列表
func parseMessages(resp string) []models.Message {
	var messages []models.Message

	blocks := strings.Split(resp, "* ")
	for _, block := range blocks {
		if !strings.Contains(block, "FETCH") {
			continue
		}
		uidMatch := uidRe.FindStringSubmatch("* " + block)
		if len(uidMatch) < 2 {
			continue
		}

		msg := models.Message{ID: uidMatch[1], IsRead: seenRe.MatchString(block)}

		if m := fromRe.FindStringSubmatch(block); len(m) > 1 {
			msg.From = &models.EmailAddr{}
			msg.From.EmailAddress.Address = decodeHeader(strings.TrimSpace(m[1]))
		}
		if m := subjRe.FindStringSubmatch(block); len(m) > 1 {
			msg.Subject = decodeHeader(strings.TrimSpace(m[1]))
		}
		if m := dateRe.FindStringSubmatch(block); len(m) > 1 {
			msg.ReceivedDateTime = strings.TrimSpace(m[1])
		}

		messages = append(messages, msg)
	}

	// 反转顺序（最新的在前）
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages
}

// parseFullMessage 解析完整邮件
func parseFullMessage(resp string) *models.Message {
	msg := &models.Message{}

	if m := fromRe.FindStringSubmatch(resp); len(m) > 1 {
		msg.From = &models.EmailAddr{}
		msg.From.EmailAddress.Address = decodeHeader(strings.TrimSpace(m[1]))
	}
	if m := toRe.FindStringSubmatch(resp); len(m) > 1 {
		msg.ToRecipients = []models.EmailAddr{{}}
		msg.ToRecipients[0].EmailAddress.Address = decodeHeader(strings.TrimSpace(m[1]))
	}
	if m := subjRe.FindStringSubmatch(resp); len(m) > 1 {
		msg.Subject = decodeHeader(strings.TrimSpace(m[1]))
	}
	if m := dateRe.FindStringSubmatch(resp); len(m) > 1 {
		msg.ReceivedDateTime = strings.TrimSpace(m[1])
	}

	// 提取正文
	body, isHTML := extractBodyWithType(resp)
	contentType := "Text"
	if isHTML {
		contentType = "HTML"
		body = sanitizeHTML(body) // 清理HTML中的脚本
	}
	msg.Body = &models.MessageBody{ContentType: contentType, Content: body}
	msg.BodyPreview = truncate(stripHTML(body), 200)

	return msg
}

// stripHTML 移除HTML标签用于预览
func stripHTML(s string) string {
	return htmlTagRe.ReplaceAllString(s, "")
}

// sanitizeHTML 清理HTML中的所有脚本内容
func sanitizeHTML(s string) string {
	// 移除script标签及内容
	s = regexp.MustCompile(`(?is)<script[\s\S]*?</script>`).ReplaceAllString(s, "")
	// 移除未闭合的script标签
	s = regexp.MustCompile(`(?i)<script[^>]*>`).ReplaceAllString(s, "")
	// 移除noscript标签
	s = regexp.MustCompile(`(?is)<noscript[\s\S]*?</noscript>`).ReplaceAllString(s, "")
	// 移除所有on*事件属性
	s = regexp.MustCompile(`(?i)\s+on\w+\s*=\s*["'][^"']*["']`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?i)\s+on\w+\s*=\s*[^\s>]+`).ReplaceAllString(s, "")
	// 移除javascript: URL
	s = regexp.MustCompile(`(?i)javascript:`).ReplaceAllString(s, "blocked:")
	// 移除vbscript: URL
	s = regexp.MustCompile(`(?i)vbscript:`).ReplaceAllString(s, "blocked:")
	// 移除data: URL中的脚本
	s = regexp.MustCompile(`(?i)data:\s*text/html`).ReplaceAllString(s, "blocked:")
	return s
}

// extractBodyWithType 提取邮件正文，返回内容和是否为HTML
func extractBodyWithType(raw string) (string, bool) {
	// 查找boundary
	boundaryMatch := boundaryRe.FindStringSubmatch(raw)

	// 如果是MIME多部分邮件
	if len(boundaryMatch) > 1 {
		boundary := boundaryMatch[1]
		parts := strings.Split(raw, "--"+boundary)

		var textContent, htmlContent string
		for _, part := range parts {
			partLower := strings.ToLower(part)

			// 查找Content-Type
			if strings.Contains(partLower, "content-type: text/plain") {
				textContent = extractPartContent(part)
			} else if strings.Contains(partLower, "content-type: text/html") {
				htmlContent = extractPartContent(part)
			}
		}

		// 优先返回HTML内容
		if htmlContent != "" {
			return htmlContent, true
		}
		if textContent != "" {
			return textContent, false
		}
	}

	// 非多部分邮件，直接提取正文
	parts := strings.SplitN(raw, "\r\n\r\n", 2)
	if len(parts) < 2 {
		parts = strings.SplitN(raw, "\n\n", 2)
	}
	if len(parts) < 2 {
		return "", false
	}

	body := parts[1]
	// 移除IMAP结束标记
	if idx := strings.LastIndex(body, ")\r\n"); idx > 0 {
		body = body[:idx]
	}
	if idx := strings.LastIndex(body, " UID"); idx > 0 {
		body = body[:idx]
	}

	content := decodeContent(parts[0], body)
	isHTML := strings.Contains(strings.ToLower(parts[0]), "text/html")
	return content, isHTML
}

// extractPartContent 从MIME部分提取内容
func extractPartContent(part string) string {
	// 分离头部和正文
	parts := strings.SplitN(part, "\r\n\r\n", 2)
	if len(parts) < 2 {
		parts = strings.SplitN(part, "\n\n", 2)
	}
	if len(parts) < 2 {
		return ""
	}

	header := parts[0]
	body := strings.TrimSpace(parts[1])

	// 移除结束boundary标记
	if idx := strings.Index(body, "\r\n--"); idx > 0 {
		body = body[:idx]
	}
	if idx := strings.Index(body, "\n--"); idx > 0 {
		body = body[:idx]
	}

	return decodeContent(header, body)
}

// decodeContent 解码邮件内容
func decodeContent(header, body string) string {
	headerLower := strings.ToLower(header)

	// 解码quoted-printable
	if strings.Contains(headerLower, "quoted-printable") {
		decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(body)))
		if err == nil {
			body = string(decoded)
		}
	}

	// 解码base64
	if strings.Contains(headerLower, "base64") {
		cleanBody := strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", ""), "\n", "")
		decoded, err := base64.StdEncoding.DecodeString(cleanBody)
		if err == nil {
			body = string(decoded)
		}
	}

	return strings.TrimSpace(body)
}

// decodeHeader 解码邮件头（RFC 2047）
func decodeHeader(s string) string {
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return decoded
}

// decodeIMAPUTF7 解码IMAP修改版UTF-7
func decodeIMAPUTF7(s string) string {
	// 简单处理：将&替换为+，-替换为空
	if !strings.Contains(s, "&") {
		return s
	}
	// 常见中文文件夹映射
	replacements := map[string]string{
		"&UXZO1mWHTvZZOQ-": "已发送邮件",
		"&V4NXPpCuTvY-":    "已删除邮件",
		"&Xn9USpCuTvY-":    "垃圾邮件",
		"&g0l6P3ux-":       "草稿",
		"&W1hoYw-":         "存档",
	}
	if decoded, ok := replacements[s]; ok {
		return decoded
	}
	return s
}

// truncate 截断字符串到指定长度
//
// 如果字符串长度超过指定值，截断并添加省略号"..."。
// 用于生成邮件预览文本，避免显示过长的内容。
//
// 参数：
//   - s: 要截断的原始字符串
//   - n: 最大保留长度（不含省略号）
//
// 返回值：
//   - string: 截断后的字符串，超长时末尾带"..."
//
// 示例：
//
//	truncate("Hello World", 5) // => "Hello..."
//	truncate("Hi", 5)          // => "Hi"
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
