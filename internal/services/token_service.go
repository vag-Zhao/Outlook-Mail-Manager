// Package services 业务服务层
//
// token_service.go OAuth2 Token刷新服务
//
// 功能说明：
// - 使用RefreshToken获取新的AccessToken
// - 与Microsoft Identity Platform交互
//
// OAuth2刷新流程：
// 1. 客户端使用RefreshToken向授权服务器请求新Token
// 2. 授权服务器验证RefreshToken有效性
// 3. 返回新的AccessToken（和可能更新的RefreshToken）
//
// Microsoft OAuth2端点：
// https://login.microsoftonline.com/common/oauth2/v2.0/token
package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// TokenResponse OAuth2令牌响应结构
//
// 对应Microsoft Identity Platform的令牌响应格式
// 成功时返回access_token等字段，失败时返回error和error_description
type TokenResponse struct {
	AccessToken  string `json:"access_token"`       // 访问令牌，用于API调用认证
	RefreshToken string `json:"refresh_token"`      // 刷新令牌，可能会更新
	ExpiresIn    int    `json:"expires_in"`         // 过期时间（秒），通常为3600（1小时）
	TokenType    string `json:"token_type"`         // 令牌类型，通常为"Bearer"
	Error        string `json:"error"`              // 错误代码（失败时）
	ErrorDesc    string `json:"error_description"`  // 错误描述（失败时）
}

// OAuth2 Scope常量定义
//
// Scope定义了应用程序请求的权限范围
// Microsoft Identity Platform使用scope来控制访问令牌的权限
const (
	// ScopeIMAP IMAP协议访问权限
	//
	// 包含两个权限：
	// - IMAP.AccessAsUser.All: 允许以用户身份通过IMAP协议访问邮箱
	// - offline_access: 允许获取refresh_token以便离线刷新访问令牌
	//
	// 注意：REST API不需要显式设置scope，使用原始token权限即可
	ScopeIMAP = "https://outlook.office.com/IMAP.AccessAsUser.All offline_access"
)

// RefreshAccessToken 刷新访问令牌（REST API，不设置scope使用原始权限）
func RefreshAccessToken(clientID, refreshToken string) (*TokenResponse, error) {
	// 尝试consumers端点（个人账户）
	token, err := refreshWithEndpoint(clientID, refreshToken, "", "consumers")
	if err != nil && strings.Contains(err.Error(), "invalid_grant") {
		// 回退到common端点（工作/学校账户）
		return refreshWithEndpoint(clientID, refreshToken, "", "common")
	}
	return token, err
}

// RefreshAccessTokenForIMAP 刷新访问令牌（IMAP scope）
func RefreshAccessTokenForIMAP(clientID, refreshToken string) (*TokenResponse, error) {
	// 尝试consumers端点（个人账户）
	token, err := refreshWithEndpoint(clientID, refreshToken, ScopeIMAP, "consumers")
	if err != nil && strings.Contains(err.Error(), "invalid_grant") {
		// 回退到common端点（工作/学校账户）
		return refreshWithEndpoint(clientID, refreshToken, ScopeIMAP, "common")
	}
	return token, err
}

// refreshWithEndpoint 使用指定端点刷新访问令牌
//
// 这是Token刷新的核心实现函数，向Microsoft Identity Platform发送刷新请求。
// 支持不同的租户端点以适配个人账户和工作/学校账户。
//
// 参数：
//   - clientID: OAuth2应用程序的客户端ID（在Azure AD中注册）
//   - refreshToken: 用于获取新访问令牌的刷新令牌
//   - scope: 请求的权限范围（IMAP需要设置，REST API传空字符串）
//   - tenant: 租户标识符，可选值：
//     - "consumers": 个人Microsoft账户（Hotmail、Outlook.com等）
//     - "common": 工作/学校账户（Office 365）
//
// 返回值：
//   - *TokenResponse: 包含新访问令牌的响应结构
//   - error: 请求失败、解析失败或Token无效时返回错误
//
// 请求格式：
//
//	POST https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token
//	Content-Type: application/x-www-form-urlencoded
//	Body: client_id=xxx&grant_type=refresh_token&refresh_token=xxx&scope=xxx
func refreshWithEndpoint(clientID, refreshToken, scope, tenant string) (*TokenResponse, error) {
	endpoint := "https://login.microsoftonline.com/" + tenant + "/oauth2/v2.0/token"

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	// 只有IMAP需要设置scope，REST API使用原始token权限
	if scope != "" {
		data.Set("scope", scope)
	}

	// 发送POST请求，Content-Type为application/x-www-form-urlencoded
	resp, err := http.Post(endpoint, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, _ := io.ReadAll(resp.Body)

	// 解析JSON响应
	var token TokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}

	// 检查是否返回错误（Token无效、过期等）
	if token.Error != "" {
		return nil, fmt.Errorf("%s: %s", token.Error, token.ErrorDesc)
	}

	return &token, nil
}
