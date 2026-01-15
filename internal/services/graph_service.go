// Package services 业务服务层
//
// graph_service.go 封装Microsoft Outlook REST API调用
//
// API文档：https://docs.microsoft.com/en-us/previous-versions/office/office-365-api/api/version-2.0/mail-rest-operations
// 基础URL：https://outlook.office.com/api/v2.0
//
// 功能说明：
// - 获取邮件文件夹列表
// - 获取/搜索邮件列表
// - 获取邮件详情和附件
// - 删除邮件、标记已读状态
package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"outlook-mail-manager/internal/models"
)

// GraphService Microsoft Outlook API服务
//
// 封装所有与Outlook REST API的交互
// 所有方法都需要有效的OAuth2访问令牌
type GraphService struct{}

// NewGraphService 创建GraphService实例
//
// 返回值：
//   - *GraphService: 服务实例
func NewGraphService() *GraphService {
	return &GraphService{}
}

// request 通用HTTP GET请求方法
//
// 封装了认证头设置、错误处理等通用逻辑
// 所有GET类型的API调用都通过此方法
//
// 参数：
//   - accessToken: OAuth2访问令牌
//   - endpoint: API端点路径（不含基础URL）
//
// 返回值：
//   - []byte: API响应体
//   - error: 请求错误或API错误（401表示Token过期）
func (s *GraphService) request(accessToken, endpoint string) ([]byte, error) {
	// 构建完整URL并创建GET请求
	req, _ := http.NewRequest("GET", "https://outlook.office.com/api/v2.0"+endpoint, nil)
	// 设置OAuth2 Bearer认证头
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 处理401未授权错误（Token过期）
	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("unauthorized: token expired")
	}
	// 处理其他非200错误
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	// 返回响应体
	return io.ReadAll(resp.Body)
}

// GetMailFolders 获取邮件文件夹列表
//
// API端点：GET /me/mailFolders
// 返回用户的所有邮件文件夹（收件箱、已发送、草稿、垃圾邮件等）
//
// 参数：
//   - accessToken: OAuth2访问令牌
//
// 返回值：
//   - []models.MailFolder: 文件夹列表，包含ID、名称、邮件数、未读数
//   - error: API调用错误
func (s *GraphService) GetMailFolders(accessToken string) ([]models.MailFolder, error) {
	// $top=50 限制返回最多50个文件夹
	data, err := s.request(accessToken, "/me/mailFolders?$top=50")
	if err != nil {
		return nil, err
	}
	// 解析OData响应格式（value数组）
	var result struct {
		Value []models.MailFolder `json:"value"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse mail folders failed: %w", err)
	}
	return result.Value, nil
}

// GetMessages 获取指定文件夹的邮件列表
//
// API端点：GET /me/mailFolders/{id}/messages
// 支持分页、排序和字段选择
//
// 参数：
//   - accessToken: OAuth2访问令牌
//   - folderID: 文件夹ID（如"inbox"、"junkemail"或GUID）
//   - skip: 跳过的邮件数（用于分页）
//   - top: 返回的邮件数（每页大小）
//
// 返回值：
//   - []models.Message: 邮件列表（按接收时间倒序）
//   - error: API调用错误
func (s *GraphService) GetMessages(accessToken, folderID string, skip, top int) ([]models.Message, error) {
	// 构建查询参数：
	// $skip: 分页偏移量
	// $top: 每页数量
	// $orderby: 按接收时间倒序
	// $select: 只返回需要的字段（优化性能）
	endpoint := fmt.Sprintf("/me/mailFolders/%s/messages?$skip=%d&$top=%d&$orderby=receivedDateTime desc&$select=id,subject,bodyPreview,from,receivedDateTime,hasAttachments,isRead",
		folderID, skip, top)
	data, err := s.request(accessToken, endpoint)
	if err != nil {
		return nil, err
	}
	var result struct {
		Value []models.Message `json:"value"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse messages failed: %w", err)
	}
	return result.Value, nil
}

// GetMessage 获取单封邮件详情
//
// API端点：GET /me/messages/{id}
// 获取邮件的完整内容，包括HTML正文
//
// 参数：
//   - accessToken: OAuth2访问令牌
//   - messageID: 邮件ID
//
// 返回值：
//   - *models.Message: 邮件详情（含完整正文）
//   - error: API调用错误
func (s *GraphService) GetMessage(accessToken, messageID string) (*models.Message, error) {
	// $select包含body字段以获取完整正文
	data, err := s.request(accessToken, "/me/messages/"+messageID+"?$select=id,subject,body,bodyPreview,from,toRecipients,receivedDateTime,hasAttachments,isRead")
	if err != nil {
		return nil, err
	}
	var msg models.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("parse message failed: %w", err)
	}
	return &msg, nil
}

// GetAttachments 获取邮件附件列表
//
// API端点：GET /me/messages/{id}/attachments
// 返回邮件的所有附件，包含Base64编码的文件内容
//
// 参数：
//   - accessToken: OAuth2访问令牌
//   - messageID: 邮件ID
//
// 返回值：
//   - []models.Attachment: 附件列表
//   - error: API调用错误
func (s *GraphService) GetAttachments(accessToken, messageID string) ([]models.Attachment, error) {
	data, err := s.request(accessToken, "/me/messages/"+messageID+"/attachments")
	if err != nil {
		return nil, err
	}
	var result struct {
		Value []models.Attachment `json:"value"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse attachments failed: %w", err)
	}
	return result.Value, nil
}

