// Package models 数据模型层
//
// account.go 账号和分组数据模型定义
//
// 本文件定义了应用的核心数据结构：
// - Account: 邮箱账号信息
// - Group: 账号分组信息
//
// 这些结构体用于：
// - 数据库记录映射
// - JSON序列化（前端通信）
// - 业务逻辑处理
package models

import "time"

// Account 邮箱账号模型
//
// 存储Outlook邮箱账号的完整信息，包括认证凭据和状态
// 对应数据库accounts表
//
// JSON标签说明：
// - omitempty: 空值时不序列化（减少传输数据量）
// - "-": 不序列化到JSON（敏感数据保护）
type Account struct {
	ID             int64      `json:"id"`                       // 账号ID，主键
	Email          string     `json:"email"`                    // 邮箱地址，唯一标识
	Password       string     `json:"password,omitempty"`       // 邮箱密码（可选，用于显示）
	ClientID       string     `json:"clientId"`                 // OAuth2客户端ID（Azure应用注册）
	RefreshToken   string     `json:"refreshToken,omitempty"`   // OAuth2刷新令牌（长期有效）
	AccessToken    string     `json:"-"`                        // OAuth2访问令牌（不传给前端，安全考虑）
	TokenExpiresAt *time.Time `json:"tokenExpiresAt,omitempty"` // 访问令牌过期时间
	GroupID        *int64     `json:"groupId,omitempty"`        // 所属分组ID（可为空）
	GroupName      string     `json:"groupName,omitempty"`      // 分组名称（JOIN查询填充）
	DisplayName    string     `json:"displayName,omitempty"`    // 显示名称
	Status         string     `json:"status"`                   // 状态：active=正常, error=异常
	Protocol       string     `json:"protocol"`                 // 协议类型：o2=REST API, imap=IMAP协议
	LastError      string     `json:"lastError,omitempty"`      // 最后一次错误信息
	CreatedAt      time.Time  `json:"createdAt"`                // 创建时间
	UpdatedAt      time.Time  `json:"updatedAt"`                // 更新时间
}

// Group 分组模型
//
// 用于组织和管理账号，支持按分组筛选和批量操作
// 对应数据库groups表
type Group struct {
	ID        int64  `json:"id"`                  // 分组ID，主键
	Name      string `json:"name"`                // 分组名称
	ParentID  *int64 `json:"parentId,omitempty"`  // 父分组ID（预留，支持嵌套分组）
	SortOrder int    `json:"sortOrder"`           // 排序顺序
	Count     int    `json:"count,omitempty"`     // 分组内账号数量（查询时计算）
}
