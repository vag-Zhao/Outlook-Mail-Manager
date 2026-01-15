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

// RefreshAccessToken 刷新访问令牌
//
// 使用RefreshToken向Microsoft OAuth2服务器请求新的AccessToken
// 这是OAuth2标准的refresh_token授权类型
//
// 请求参数：
// - client_id: Azure应用注册的客户端ID
// - grant_type: "refresh_token"（固定值）
// - refresh_token: 之前获取的刷新令牌
//
// 注意：不设置scope参数，使用原始token的权限范围（outlook.office.com）
//
// 参数：
//   - clientID: OAuth2客户端ID（Azure应用注册）
//   - refreshToken: 刷新令牌
//
// 返回值：
//   - *TokenResponse: 包含新AccessToken的响应
//   - error: 请求失败或Token无效时返回错误
func RefreshAccessToken(clientID, refreshToken string) (*TokenResponse, error) {
	// Microsoft Identity Platform v2.0 令牌端点
	// "common"表示支持任何Microsoft账户（个人/工作/学校）
	endpoint := "https://login.microsoftonline.com/common/oauth2/v2.0/token"

	// 构建表单数据
	data := url.Values{}
	data.Set("client_id", clientID)           // 客户端ID
	data.Set("grant_type", "refresh_token")   // 授权类型：刷新令牌
	data.Set("refresh_token", refreshToken)   // 刷新令牌
	// 不设置scope，使用原始token的权限（outlook.office.com）

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
