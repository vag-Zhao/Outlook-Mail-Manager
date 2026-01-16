// Package main 邮箱管家应用主包
//
// 本文件是Wails桌面应用的核心控制器，负责：
// - 前端Vue组件与后端Go服务之间的桥接
// - OAuth2 Token的缓存管理和自动刷新
// - 账号、分组、邮件操作的API暴露
//
// 架构说明：
// App结构体作为Wails绑定对象，其所有公开方法都可被前端JavaScript调用
// 内部通过服务层(services)访问数据库和外部API
package main

import (
	"context"
	"log"
	"os"
	"outlook-mail-manager/internal/database"
	"outlook-mail-manager/internal/models"
	"outlook-mail-manager/internal/services"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// tokenCache Token缓存结构
//
// 用于在内存中缓存已获取的访问令牌，避免频繁刷新Token
// 缓存策略：Token在过期前1分钟视为无效，触发刷新
type tokenCache struct {
	token     string    // OAuth2访问令牌(Access Token)
	expiresAt time.Time // 令牌过期时间
}

// App 应用核心结构体
//
// 作为Wails框架的绑定对象，所有公开方法都会暴露给前端调用
// 内部聚合了所有业务服务，实现了关注点分离
type App struct {
	ctx        context.Context           // Wails运行时上下文，用于调用系统对话框等功能
	accountSvc *services.AccountService  // 账号服务：处理账号的CRUD操作
	groupSvc   *services.GroupService    // 分组服务：处理分组的CRUD操作
	graphSvc   *services.GraphService    // Graph服务：封装Microsoft Outlook API调用
	imapSvc    *services.IMAPService     // IMAP服务：用于Hotmail等个人账户
	tokenMu    sync.RWMutex              // Token缓存读写锁，保证并发安全
	tokens     map[int64]*tokenCache     // Token缓存映射表，key为账号ID
	imapTokens map[int64]*tokenCache     // IMAP Token缓存（使用不同scope）
}

// NewApp 创建应用实例
//
// 初始化所有业务服务和Token缓存
// 此函数在main.go中被调用，创建的实例会绑定到Wails运行时
//
// 返回值：
//   - *App: 初始化完成的应用实例
func NewApp() *App {
	return &App{
		accountSvc: services.NewAccountService(), // 初始化账号服务
		groupSvc:   services.NewGroupService(),   // 初始化分组服务
		graphSvc:   services.NewGraphService(),   // 初始化Graph API服务
		imapSvc:    services.NewIMAPService(),    // 初始化IMAP服务
		tokens:     make(map[int64]*tokenCache),  // 初始化空的Token缓存
		imapTokens: make(map[int64]*tokenCache),  // 初始化IMAP Token缓存
	}
}

// startup Wails应用启动回调
//
// 在应用窗口显示前由Wails框架自动调用
// 负责初始化数据库连接和执行数据迁移
//
// 参数：
//   - ctx: Wails运行时上下文，包含窗口操作、对话框等功能
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx // 保存上下文供后续使用（如文件对话框）
	if err := database.Init(); err != nil {
		// 数据库初始化失败时记录错误日志，但不阻止应用启动
		runtime.LogError(ctx, "database init failed: "+err.Error())
	}
}

// shutdown Wails应用关闭回调
//
// 在应用窗口关闭时由Wails框架自动调用
// 负责清理资源，关闭数据库连接
//
// 参数：
//   - ctx: Wails运行时上下文
func (a *App) shutdown(ctx context.Context) {
	database.Close() // 关闭SQLite数据库连接
}

// ============================================================================
// 账号管理API - 提供账号的增删改查操作
// ============================================================================

// ImportAccounts 批量导入账号
//
// 解析用户输入的文本内容，批量创建账号记录
// 支持的格式：邮箱----密码----ClientID----RefreshToken 或 Tab分隔
//
// 参数：
//   - content: 包含账号信息的多行文本
//
// 返回值：
//   - int: 成功导入的账号数量
//   - error: 导入过程中的错误
func (a *App) ImportAccounts(content string) (int, error) {
	return a.accountSvc.Import(content)
}

// GetAccounts 获取账号列表
//
// 根据分组ID筛选账号，若groupID为nil则返回所有账号
//
// 参数：
//   - groupID: 分组ID指针，nil表示不筛选
//
// 返回值：
//   - []models.Account: 账号列表
//   - error: 查询错误
func (a *App) GetAccounts(groupID *int64) ([]models.Account, error) {
	return a.accountSvc.List(groupID)
}

// DeleteAccount 删除单个账号
//
// 参数：
//   - id: 要删除的账号ID
//
// 返回值：
//   - error: 删除失败时返回错误
func (a *App) DeleteAccount(id int64) error {
	return a.accountSvc.Delete(id)
}

// DeleteAccounts 批量删除账号
//
// 遍历ID列表逐个删除，遇到错误立即返回
//
// 参数：
//   - ids: 要删除的账号ID列表
//
// 返回值：
//   - error: 删除过程中的第一个错误
func (a *App) DeleteAccounts(ids []int64) error {
	for _, id := range ids {
		if err := a.accountSvc.Delete(id); err != nil {
			return err
		}
	}
	return nil
}

// MoveAccountsToGroup 批量移动账号到指定分组
//
// 参数：
//   - ids: 要移动的账号ID列表
//   - groupID: 目标分组ID
//
// 返回值：
//   - error: 移动过程中的第一个错误
func (a *App) MoveAccountsToGroup(ids []int64, groupID int64) error {
	for _, id := range ids {
		if err := a.accountSvc.UpdateGroup(id, groupID); err != nil {
			return err
		}
	}
	return nil
}

// CheckAccountToken 检测账号Token有效性
//
// 强制刷新Token以验证账号的RefreshToken是否仍然有效
//
// 参数：
//   - accountID: 要检测的账号ID
//
// 返回值：
//   - bool: Token是否有效
//   - error: 刷新Token时的错误信息
func (a *App) CheckAccountToken(accountID int64) (bool, error) {
	_, err := a.getToken(accountID, true)
	return err == nil, err
}

// MoveAccountToGroup 移动单个账号到指定分组
//
// 参数：
//   - accountID: 要移动的账号ID
//   - groupID: 目标分组ID
//
// 返回值：
//   - error: 更新失败时返回错误
func (a *App) MoveAccountToGroup(accountID, groupID int64) error {
	return a.accountSvc.UpdateGroup(accountID, groupID)
}

// ============================================================================
// 分组管理API - 提供分组的增删改查操作
// ============================================================================

// GetGroups 获取所有分组列表
//
// 返回值：
//   - []models.Group: 分组列表，包含每个分组的账号数量
//   - error: 查询错误
func (a *App) GetGroups() ([]models.Group, error) {
	return a.groupSvc.List()
}

// CreateGroup 创建新分组
//
// 参数：
//   - name: 分组名称
//
// 返回值：
//   - *models.Group: 创建成功的分组对象
//   - error: 创建失败时返回错误
func (a *App) CreateGroup(name string) (*models.Group, error) {
	return a.groupSvc.Create(name, nil) // 第二个参数为父分组ID，暂不支持嵌套分组
}

// DeleteGroup 删除分组
//
// 删除分组后，该分组下的账号会被设置为无分组状态(group_id=NULL)
//
// 参数：
//   - id: 要删除的分组ID
//
// 返回值：
//   - error: 删除失败时返回错误
func (a *App) DeleteGroup(id int64) error {
	return a.groupSvc.Delete(id)
}

// ClearGroup 清空分组内所有账号
//
// 删除指定分组下的所有账号记录
//
// 参数：
//   - groupID: 要清空的分组ID
//
// 返回值：
//   - error: 删除失败时返回错误
func (a *App) ClearGroup(groupID int64) error {
	return a.accountSvc.DeleteByGroup(groupID)
}

// ============================================================================
// 邮件操作API - 提供邮件的查看等操作
// 所有邮件操作都需要有效的OAuth2 Token，支持Token过期自动重试
// ============================================================================

// GetMailFolders 获取邮箱文件夹列表
//
// 策略：已标记imap的直接用IMAP，否则先尝试REST API，失败后回退到IMAP并标记
func (a *App) GetMailFolders(accountID int64) ([]models.MailFolder, error) {
	log.Printf("[App] GetMailFolders 开始 - accountID: %d", accountID)

	account, err := a.accountSvc.GetByID(accountID)
	if err != nil {
		log.Printf("[App] GetByID 失败: %v", err)
		return nil, err
	}
	log.Printf("[App] 账号信息: email=%s, protocol=%s, status=%s", account.Email, account.Protocol, account.Status)

	// 已标记为 IMAP 的账号直接使用 IMAP
	if account.Protocol == "imap" {
		log.Printf("[App] 账号已标记为 IMAP，直接使用 IMAP 协议")
		imapToken, err := a.getIMAPToken(accountID, false)
		if err != nil {
			log.Printf("[App] getIMAPToken 失败: %v", err)
			return nil, err
		}
		log.Printf("[App] IMAP Token 获取成功，调用 imapSvc.GetMailFolders")
		return a.imapSvc.GetMailFolders(account.Email, imapToken)
	}

	// 先尝试 REST API
	log.Printf("[App] 尝试 REST API (O2)...")
	if token, err := a.ensureValidToken(accountID); err == nil {
		log.Printf("[App] O2 Token 获取成功，调用 graphSvc.GetMailFolders")
		if result, err := a.graphSvc.GetMailFolders(token); err == nil {
			log.Printf("[App] O2 成功，返回 %d 个文件夹", len(result))
			return result, nil
		} else {
			log.Printf("[App] O2 GetMailFolders 失败: %v", err)
			if strings.Contains(err.Error(), "unauthorized") {
				log.Printf("[App] Token 过期，清除缓存并重试...")
				a.clearTokenCache(accountID)
				if token, err = a.getToken(accountID, true); err == nil {
					log.Printf("[App] 重新获取 Token 成功，再次调用 graphSvc.GetMailFolders")
					if result, err := a.graphSvc.GetMailFolders(token); err == nil {
						log.Printf("[App] O2 重试成功，返回 %d 个文件夹", len(result))
						return result, nil
					} else {
						log.Printf("[App] O2 重试失败: %v", err)
					}
				} else {
					log.Printf("[App] 重新获取 Token 失败: %v", err)
				}
			}
		}
	} else {
		log.Printf("[App] ensureValidToken 失败: %v", err)
	}

	// REST API 失败，回退到 IMAP 并标记
	log.Printf("[App] O2 失败，回退到 IMAP...")
	imapToken, err := a.getIMAPToken(accountID, false)
	if err != nil {
		log.Printf("[App] getIMAPToken 失败: %v", err)
		return nil, err
	}
	log.Printf("[App] IMAP Token 获取成功，调用 imapSvc.GetMailFolders")
	result, err := a.imapSvc.GetMailFolders(account.Email, imapToken)
	if err == nil {
		log.Printf("[App] IMAP 成功，返回 %d 个文件夹，标记账号为 IMAP", len(result))
		a.accountSvc.UpdateProtocol(accountID, "imap")
		runtime.EventsEmit(a.ctx, "protocol-updated", accountID, "imap")
	} else {
		log.Printf("[App] IMAP 也失败: %v", err)
	}
	return result, err
}

// GetMessages 获取指定文件夹的邮件列表
//
// 策略：已标记imap的直接用IMAP，否则先尝试REST API，失败后回退到IMAP并标记
func (a *App) GetMessages(accountID int64, folderID string, page int) ([]models.Message, error) {
	log.Printf("[App] GetMessages 开始 - accountID: %d, folderID: %s, page: %d", accountID, folderID, page)

	account, err := a.accountSvc.GetByID(accountID)
	if err != nil {
		log.Printf("[App] GetByID 失败: %v", err)
		return nil, err
	}
	log.Printf("[App] 账号: email=%s, protocol=%s", account.Email, account.Protocol)

	// 已标记为 IMAP 的账号直接使用 IMAP
	if account.Protocol == "imap" {
		log.Printf("[App] 账号已标记为 IMAP，直接使用 IMAP")
		imapToken, err := a.getIMAPToken(accountID, false)
		if err != nil {
			log.Printf("[App] getIMAPToken 失败: %v", err)
			return nil, err
		}
		log.Printf("[App] 调用 imapSvc.GetMessages")
		return a.imapSvc.GetMessages(account.Email, imapToken, folderID, page*20, 20)
	}

	// 先尝试 REST API
	log.Printf("[App] 尝试 REST API (O2)...")
	if token, err := a.ensureValidToken(accountID); err == nil {
		log.Printf("[App] O2 Token 获取成功")
		if result, err := a.graphSvc.GetMessages(token, folderID, page*20, 20); err == nil {
			log.Printf("[App] O2 成功，返回 %d 封邮件", len(result))
			return result, nil
		} else {
			log.Printf("[App] O2 GetMessages 失败: %v", err)
			if strings.Contains(err.Error(), "unauthorized") {
				log.Printf("[App] Token 过期，重试...")
				a.clearTokenCache(accountID)
				if token, err = a.getToken(accountID, true); err == nil {
					if result, err := a.graphSvc.GetMessages(token, folderID, page*20, 20); err == nil {
						log.Printf("[App] O2 重试成功")
						return result, nil
					}
				}
			}
		}
	} else {
		log.Printf("[App] ensureValidToken 失败: %v", err)
	}

	// REST API 失败，回退到 IMAP 并标记
	log.Printf("[App] O2 失败，回退到 IMAP...")
	imapToken, err := a.getIMAPToken(accountID, false)
	if err != nil {
		log.Printf("[App] getIMAPToken 失败: %v", err)
		return nil, err
	}
	result, err := a.imapSvc.GetMessages(account.Email, imapToken, folderID, page*20, 20)
	if err == nil {
		log.Printf("[App] IMAP 成功，返回 %d 封邮件，标记账号为 IMAP", len(result))
		a.accountSvc.UpdateProtocol(accountID, "imap")
		runtime.EventsEmit(a.ctx, "protocol-updated", accountID, "imap")
	} else {
		log.Printf("[App] IMAP 也失败: %v", err)
	}
	return result, err
}

// GetMessageDetail 获取邮件详情
//
// 策略：已标记imap的直接用IMAP，否则先尝试REST API，失败后回退到IMAP并标记
func (a *App) GetMessageDetail(accountID int64, messageID string, folderID string) (*models.Message, error) {
	account, err := a.accountSvc.GetByID(accountID)
	if err != nil {
		return nil, err
	}

	var msg *models.Message

	// 已标记为 IMAP 的账号直接使用 IMAP
	if account.Protocol == "imap" {
		imapToken, err := a.getIMAPToken(accountID, false)
		if err != nil {
			return nil, err
		}
		if folderID == "" {
			folderID = "inbox"
		}
		msg, err = a.imapSvc.GetMessage(account.Email, imapToken, folderID, messageID)
		if err != nil {
			return nil, err
		}
		goto sanitize
	}

	// 先尝试 REST API
	if token, err := a.ensureValidToken(accountID); err == nil {
		if msg, err = a.graphSvc.GetMessage(token, messageID); err == nil {
			goto sanitize
		} else if strings.Contains(err.Error(), "unauthorized") {
			a.clearTokenCache(accountID)
			if token, err = a.getToken(accountID, true); err == nil {
				if msg, err = a.graphSvc.GetMessage(token, messageID); err == nil {
					goto sanitize
				}
			}
		}
	}

	// REST API 失败，回退到 IMAP 并标记
	{
		imapToken, err := a.getIMAPToken(accountID, false)
		if err != nil {
			return nil, err
		}
		if folderID == "" {
			folderID = "inbox"
		}
		msg, err = a.imapSvc.GetMessage(account.Email, imapToken, folderID, messageID)
		if err != nil {
			return nil, err
		}
		a.accountSvc.UpdateProtocol(accountID, "imap")
		runtime.EventsEmit(a.ctx, "protocol-updated", accountID, "imap")
	}

sanitize:
	// 统一清理HTML内容中的脚本
	if msg != nil && msg.Body != nil && strings.ToLower(msg.Body.ContentType) == "html" {
		msg.Body.Content = sanitizeHTML(msg.Body.Content)
	}
	return msg, nil
}

// sanitizeHTML 清理HTML中的所有脚本内容
func sanitizeHTML(s string) string {
	// 移除script标签及内容
	s = regexp.MustCompile(`(?is)<script[\s\S]*?</script>`).ReplaceAllString(s, "")
	// 移除未闭合的script标签
	s = regexp.MustCompile(`(?i)<script[^>]*>`).ReplaceAllString(s, "")
	// 移除noscript标签
	s = regexp.MustCompile(`(?is)<noscript[\s\S]*?</noscript>`).ReplaceAllString(s, "")
	// 移除iframe标签（可能包含脚本）
	s = regexp.MustCompile(`(?is)<iframe[\s\S]*?</iframe>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?i)<iframe[^>]*>`).ReplaceAllString(s, "")
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

// GetAttachments 获取邮件附件列表
//
// 策略：已标记imap的直接返回空，否则尝试REST API
func (a *App) GetAttachments(accountID int64, messageID string) ([]models.Attachment, error) {
	account, err := a.accountSvc.GetByID(accountID)
	if err != nil {
		return nil, err
	}

	// 已标记为 IMAP 的账号，附件已在GetMessage中解析
	if account.Protocol == "imap" {
		return []models.Attachment{}, nil
	}

	// 尝试 REST API
	if token, err := a.ensureValidToken(accountID); err == nil {
		if result, err := a.graphSvc.GetAttachments(token, messageID); err == nil {
			return result, nil
		} else if strings.Contains(err.Error(), "unauthorized") {
			a.clearTokenCache(accountID)
			if token, err = a.getToken(accountID, true); err == nil {
				if result, err := a.graphSvc.GetAttachments(token, messageID); err == nil {
					return result, nil
				}
			}
		}
	}

	// REST API 失败，返回空（IMAP附件已在GetMessage中解析）
	return []models.Attachment{}, nil
}

// ============================================================================
// Token管理 - OAuth2访问令牌的缓存、刷新和验证
// 采用三级缓存策略：内存缓存 -> 数据库缓存 -> 远程刷新
// ============================================================================

// ensureValidToken 确保获取有效的访问令牌
//
// 内部方法，封装getToken的非强制刷新调用
//
// 参数：
//   - accountID: 账号ID
//
// 返回值：
//   - string: 有效的访问令牌
//   - error: 获取失败时返回错误
func (a *App) ensureValidToken(accountID int64) (string, error) {
	return a.getToken(accountID, false)
}

// getToken 获取访问令牌（核心Token管理方法）
//
// Token获取策略（三级缓存）：
// 1. 内存缓存：检查tokens map中是否有未过期的Token
// 2. 数据库缓存：检查accounts表中存储的Token是否有效
// 3. 远程刷新：使用RefreshToken向Microsoft服务器请求新Token
//
// Token有效性判断：过期时间必须大于当前时间+1分钟（预留缓冲）
//
// 参数：
//   - accountID: 账号ID
//   - forceRefresh: 是否强制刷新（跳过缓存直接请求新Token）
//
// 返回值：
//   - string: 访问令牌
//   - error: 获取失败时返回错误（如RefreshToken失效）
func (a *App) getToken(accountID int64, forceRefresh bool) (string, error) {
	// 非强制刷新时，先检查内存缓存
	if !forceRefresh {
		a.tokenMu.RLock() // 读锁，允许并发读取
		if cached, ok := a.tokens[accountID]; ok && cached.expiresAt.After(time.Now().Add(time.Minute)) {
			a.tokenMu.RUnlock()
			return cached.token, nil // 命中内存缓存，直接返回
		}
		a.tokenMu.RUnlock()
	}

	// 从数据库获取账号信息
	account, err := a.accountSvc.GetByID(accountID)
	if err != nil {
		return "", err
	}

	// 非强制刷新时，检查数据库中存储的Token是否有效
	if !forceRefresh && account.AccessToken != "" && account.TokenExpiresAt != nil && account.TokenExpiresAt.After(time.Now().Add(time.Minute)) {
		// 将数据库Token同步到内存缓存
		a.tokenMu.Lock()
		a.tokens[accountID] = &tokenCache{token: account.AccessToken, expiresAt: *account.TokenExpiresAt}
		a.tokenMu.Unlock()
		return account.AccessToken, nil
	}

	// 缓存未命中或强制刷新，调用Microsoft OAuth2接口刷新Token
	tokenResp, err := services.RefreshAccessToken(account.ClientID, account.RefreshToken)
	if err != nil {
		// Token刷新失败，更新账号状态为error
		a.accountSvc.UpdateStatus(accountID, "error", err.Error())
		return "", err
	}

	// 计算新Token的过期时间
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	// 持久化新Token到数据库
	a.accountSvc.UpdateToken(accountID, tokenResp.AccessToken, tokenResp.RefreshToken, expiresAt)
	// 更新账号状态为active
	a.accountSvc.UpdateStatus(accountID, "active", "")

	// 同步更新内存缓存
	a.tokenMu.Lock()
	a.tokens[accountID] = &tokenCache{token: tokenResp.AccessToken, expiresAt: expiresAt}
	a.tokenMu.Unlock()

	return tokenResp.AccessToken, nil
}

// clearTokenCache 清除指定账号的Token缓存
//
// 当API返回401未授权错误时调用，强制下次请求重新获取Token
//
// 参数：
//   - accountID: 要清除缓存的账号ID
func (a *App) clearTokenCache(accountID int64) {
	a.tokenMu.Lock()
	delete(a.tokens, accountID)     // 从map中删除缓存项
	delete(a.imapTokens, accountID) // 同时清除IMAP缓存
	a.tokenMu.Unlock()
}

// getIMAPToken 获取IMAP协议专用的访问令牌
//
// IMAP协议需要特定的scope权限，与REST API使用的Token不同。
// 本方法实现了Token的缓存和自动刷新机制。
//
// 缓存策略：
//  1. 检查内存缓存是否有有效Token（距过期时间>1分钟）
//  2. 缓存命中则直接返回，避免重复刷新
//  3. 缓存未命中则调用Microsoft OAuth2 API刷新Token
//  4. 刷新成功后更新数据库和内存缓存
//
// 参数：
//   - accountID: 账号ID，用于查找账号信息和缓存Token
//   - forceRefresh: 是否强制刷新，true时跳过缓存检查
//
// 返回值：
//   - string: 有效的IMAP访问令牌
//   - error: 账号不存在或Token刷新失败时返回错误
//
// 注意：刷新失败时会将账号状态标记为"error"并记录错误信息
func (a *App) getIMAPToken(accountID int64, forceRefresh bool) (string, error) {
	if !forceRefresh {
		a.tokenMu.RLock()
		if cached, ok := a.imapTokens[accountID]; ok && cached.expiresAt.After(time.Now().Add(time.Minute)) {
			a.tokenMu.RUnlock()
			return cached.token, nil
		}
		a.tokenMu.RUnlock()
	}

	account, err := a.accountSvc.GetByID(accountID)
	if err != nil {
		return "", err
	}

	// 使用IMAP专用scope刷新Token
	tokenResp, err := services.RefreshAccessTokenForIMAP(account.ClientID, account.RefreshToken)
	if err != nil {
		a.accountSvc.UpdateStatus(accountID, "error", err.Error())
		return "", err
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	// 保存新的token和refresh token到数据库
	a.accountSvc.UpdateToken(accountID, tokenResp.AccessToken, tokenResp.RefreshToken, expiresAt)
	a.accountSvc.UpdateStatus(accountID, "active", "")

	a.tokenMu.Lock()
	a.imapTokens[accountID] = &tokenCache{token: tokenResp.AccessToken, expiresAt: expiresAt}
	a.tokenMu.Unlock()

	return tokenResp.AccessToken, nil
}

// ============================================================================
// 系统功能 - 文件操作等系统级功能
// ============================================================================

// SaveFile 保存文件对话框
//
// 弹出系统文件保存对话框，让用户选择保存位置
// 用于导出账号信息等功能
//
// 参数：
//   - content: 要保存的文件内容
//
// 返回值：
//   - bool: 是否保存成功（用户取消返回false）
//   - error: 文件写入错误
func (a *App) SaveFile(content string) (bool, error) {
	// 调用Wails运行时显示保存对话框
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: "accounts.txt", // 默认文件名
		Filters: []runtime.FileFilter{
			{DisplayName: "Text Files", Pattern: "*.txt"}, // 文件类型过滤器
		},
	})
	// 用户取消或发生错误
	if err != nil || path == "" {
		return false, err
	}
	// 写入文件内容
	return true, os.WriteFile(path, []byte(content), 0644)
}
