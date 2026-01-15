// Package utils 工具函数包
//
// parser.go 账号文本解析工具
//
// 功能说明：
// - 解析用户导入的账号文本
// - 支持多种分隔符格式
// - 批量解析多行文本
//
// 支持的文本格式：
// 1. 四横线分隔：邮箱----密码----ClientID----RefreshToken----分组名
// 2. Tab分隔：邮箱\t密码\tClientID\tRefreshToken\t分组名
//
// 字段说明：
// - 邮箱（必填）：Outlook邮箱地址
// - 密码（必填）：邮箱密码（用于显示，非认证用）
// - ClientID（必填）：Azure应用注册的客户端ID
// - RefreshToken（必填）：OAuth2刷新令牌
// - 分组名（可选）：账号所属分组，默认为"默认分组"
package utils

import (
	"fmt"
	"outlook-mail-manager/internal/models"
	"strings"
)

// ParseAccountLine 解析单行账号文本
//
// 支持两种分隔符：
// - "----"（四横线）：常见的账号导出格式
// - "\t"（Tab）：Excel/表格复制格式
//
// 参数：
//   - line: 单行账号文本
//
// 返回值：
//   - *models.Account: 解析成功的账号对象
//   - string: 分组名称（默认为"默认分组"）
//   - error: 解析失败时返回错误（空行或字段不足）
func ParseAccountLine(line string) (*models.Account, string, error) {
	// 去除首尾空白字符
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, "", fmt.Errorf("empty line")
	}

	// 根据分隔符类型拆分字段
	var parts []string
	if strings.Contains(line, "----") {
		// 四横线分隔格式
		parts = strings.Split(line, "----")
	} else {
		// Tab分隔格式（Excel复制）
		parts = strings.Split(line, "\t")
	}

	// 验证必填字段数量（至少4个：邮箱、密码、ClientID、RefreshToken）
	if len(parts) < 4 {
		return nil, "", fmt.Errorf("invalid format: need 4 fields, got %d", len(parts))
	}

	// 解析分组名（第5个字段，可选）
	groupName := "默认分组"
	if len(parts) >= 5 && strings.TrimSpace(parts[4]) != "" {
		groupName = strings.TrimSpace(parts[4])
	}

	// 构建账号对象
	return &models.Account{
		Email:        strings.TrimSpace(parts[0]), // 邮箱地址
		Password:     strings.TrimSpace(parts[1]), // 密码
		ClientID:     strings.TrimSpace(parts[2]), // OAuth2客户端ID
		RefreshToken: strings.TrimSpace(parts[3]), // OAuth2刷新令牌
		Status:       "active",                    // 默认状态为active
	}, groupName, nil
}

// ParseAccountsText 批量解析账号文本
//
// 按行拆分文本，逐行解析账号信息
// 跳过空行，记录解析错误但不中断处理
//
// 参数：
//   - text: 包含多行账号信息的文本
//
// 返回值：
//   - []*models.Account: 成功解析的账号列表
//   - []string: 对应的分组名称列表（与账号列表一一对应）
//   - []error: 解析错误列表（包含行号信息）
func ParseAccountsText(text string) ([]*models.Account, []string, []error) {
	// 按换行符拆分为多行
	lines := strings.Split(text, "\n")
	var accounts []*models.Account
	var groups []string
	var errors []error

	// 逐行解析
	for i, line := range lines {
		// 跳过空行
		if strings.TrimSpace(line) == "" {
			continue
		}
		// 解析单行
		acc, group, err := ParseAccountLine(line)
		if err != nil {
			// 记录错误（包含行号，便于用户定位问题）
			errors = append(errors, fmt.Errorf("line %d: %w", i+1, err))
			continue
		}
		// 添加到结果列表
		accounts = append(accounts, acc)
		groups = append(groups, group)
	}
	return accounts, groups, errors
}
