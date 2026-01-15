// Package models 数据模型层
//
// mail.go 邮件相关数据模型定义
//
// 本文件定义了与Microsoft Outlook API交互的数据结构：
// - MailFolder: 邮件文件夹
// - Message: 邮件消息
// - MessageBody: 邮件正文
// - EmailAddr: 邮件地址
// - Attachment: 邮件附件
//
// 这些结构体的字段与Outlook REST API的JSON响应字段一一对应
// 参考文档：https://docs.microsoft.com/en-us/previous-versions/office/office-365-api/api/version-2.0/mail-rest-operations
package models

// MailFolder 邮件文件夹模型
//
// 对应Outlook API的MailFolder资源
// 常见文件夹：Inbox(收件箱)、SentItems(已发送)、Drafts(草稿)、
// DeletedItems(已删除)、JunkEmail(垃圾邮件)
type MailFolder struct {
	ID              string `json:"id"`              // 文件夹唯一标识（可能是GUID或预定义名称如"inbox"）
	DisplayName     string `json:"displayName"`     // 显示名称（如"Inbox"、"Sent Items"）
	TotalItemCount  int    `json:"totalItemCount"`  // 文件夹内邮件总数
	UnreadItemCount int    `json:"unreadItemCount"` // 未读邮件数量
}

// Message 邮件消息模型
//
// 对应Outlook API的Message资源
// 包含邮件的基本信息和内容
type Message struct {
	ID               string       `json:"id"`               // 邮件唯一标识（GUID）
	Subject          string       `json:"subject"`          // 邮件主题
	BodyPreview      string       `json:"bodyPreview"`      // 正文预览（纯文本，约255字符）
	Body             *MessageBody `json:"body,omitempty"`   // 完整正文（仅在获取详情时返回）
	From             *EmailAddr   `json:"from,omitempty"`   // 发件人地址
	ToRecipients     []EmailAddr  `json:"toRecipients,omitempty"` // 收件人列表
	ReceivedDateTime string       `json:"receivedDateTime"` // 接收时间（ISO 8601格式）
	HasAttachments   bool         `json:"hasAttachments"`   // 是否有附件
	IsRead           bool         `json:"isRead"`           // 是否已读
}

// MessageBody 邮件正文模型
//
// 包含邮件的完整正文内容
type MessageBody struct {
	ContentType string `json:"contentType"` // 内容类型："Text"或"HTML"
	Content     string `json:"content"`     // 正文内容（纯文本或HTML）
}

// EmailAddr 邮件地址模型
//
// Outlook API的邮件地址采用嵌套结构
// 外层包含emailAddress对象，内层包含name和address
type EmailAddr struct {
	EmailAddress struct {
		Name    string `json:"name"`    // 显示名称（如"张三"）
		Address string `json:"address"` // 邮箱地址（如"zhangsan@outlook.com"）
	} `json:"emailAddress"`
}

// Attachment 邮件附件模型
//
// 对应Outlook API的Attachment资源
// 支持文件附件（FileAttachment）
type Attachment struct {
	ID           string `json:"id"`                     // 附件唯一标识
	Name         string `json:"name"`                   // 文件名（如"document.pdf"）
	ContentType  string `json:"contentType"`            // MIME类型（如"application/pdf"）
	Size         int    `json:"size"`                   // 文件大小（字节）
	ContentBytes string `json:"contentBytes,omitempty"` // 文件内容（Base64编码）
}
