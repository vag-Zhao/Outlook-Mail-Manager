// Package services 业务服务层
//
// account_service.go 账号服务
//
// 功能说明：
// - 账号的CRUD操作（增删改查）
// - 批量导入账号（解析文本格式）
// - Token和状态更新
// - 分组关联管理
package services

import (
	"database/sql"
	"outlook-mail-manager/internal/database"
	"outlook-mail-manager/internal/models"
	"outlook-mail-manager/internal/utils"
	"time"
)

// AccountService 账号服务
//
// 提供账号相关的所有数据库操作
// 所有方法都直接操作SQLite数据库
type AccountService struct{}

// NewAccountService 创建账号服务实例
//
// 返回值：
//   - *AccountService: 服务实例
func NewAccountService() *AccountService {
	return &AccountService{}
}

// List 获取账号列表
//
// 支持按分组筛选，返回账号的完整信息（含分组名称）
// 使用LEFT JOIN关联groups表获取分组名称
//
// 参数：
//   - groupID: 分组ID指针，nil表示查询所有账号
//
// 返回值：
//   - []models.Account: 账号列表，按ID倒序排列（最新的在前）
//   - error: 数据库查询错误
func (s *AccountService) List(groupID *int64) ([]models.Account, error) {
	// SQL查询：关联accounts和groups表
	// COALESCE处理NULL值，提供默认值
	query := `SELECT a.id, a.email, COALESCE(a.password,''), a.client_id, COALESCE(a.refresh_token,''), COALESCE(a.access_token,''),
		a.token_expires_at, a.group_id, COALESCE(g.name, '默认分组'), COALESCE(a.display_name,''), COALESCE(a.status,'active'),
		COALESCE(a.protocol,'o2'), COALESCE(a.last_error,''), a.created_at, a.updated_at
		FROM accounts a LEFT JOIN groups g ON a.group_id = g.id`
	args := []interface{}{}
	// 可选的分组筛选条件
	if groupID != nil {
		query += " WHERE a.group_id = ?"
		args = append(args, *groupID)
	}
	query += " ORDER BY a.id DESC" // 最新账号排在前面

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 遍历结果集，构建账号列表
	var accounts []models.Account
	for rows.Next() {
		var a models.Account
		var tokenExp sql.NullString   // Token过期时间可能为NULL
		var grpID sql.NullInt64       // 分组ID可能为NULL
		var createdAt, updatedAt sql.NullString
		err := rows.Scan(&a.ID, &a.Email, &a.Password, &a.ClientID, &a.RefreshToken, &a.AccessToken,
			&tokenExp, &grpID, &a.GroupName, &a.DisplayName, &a.Status, &a.Protocol, &a.LastError, &createdAt, &updatedAt)
		if err != nil {
			continue // 跳过解析失败的行
		}
		// 处理可空的分组ID
		if grpID.Valid {
			a.GroupID = &grpID.Int64
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

// GetByID 根据ID获取单个账号详情
//
// 用于获取账号的完整信息，包括Token和过期时间
// 主要用于Token刷新流程
//
// 参数：
//   - id: 账号ID
//
// 返回值：
//   - *models.Account: 账号详情
//   - error: 账号不存在或数据库错误
func (s *AccountService) GetByID(id int64) (*models.Account, error) {
	var a models.Account
	// 使用sql.NullXxx类型处理可空字段
	var tokenExp, displayName, lastErr, accessToken, protocol sql.NullString
	var grpID sql.NullInt64
	err := database.DB.QueryRow(`SELECT id, email, COALESCE(password,''), client_id, COALESCE(refresh_token,''), access_token,
		token_expires_at, group_id, display_name, COALESCE(status,'active'), protocol, last_error FROM accounts WHERE id = ?`, id).
		Scan(&a.ID, &a.Email, &a.Password, &a.ClientID, &a.RefreshToken, &accessToken,
			&tokenExp, &grpID, &displayName, &a.Status, &protocol, &lastErr)
	if err != nil {
		return nil, err
	}
	// 处理可空字段
	if grpID.Valid {
		a.GroupID = &grpID.Int64
	}
	if displayName.Valid {
		a.DisplayName = displayName.String
	}
	if accessToken.Valid {
		a.AccessToken = accessToken.String
	}
	if protocol.Valid {
		a.Protocol = protocol.String
	}
	// 解析Token过期时间（RFC3339格式）
	if tokenExp.Valid {
		t, _ := time.Parse(time.RFC3339, tokenExp.String)
		a.TokenExpiresAt = &t
	}
	return &a, nil
}

// Import 批量导入账号
//
// 解析用户输入的文本，批量创建或更新账号
// 使用INSERT OR REPLACE实现存在则更新、不存在则插入
//
// 支持的文本格式：
// - 邮箱----密码----ClientID----RefreshToken----分组名
// - 邮箱\t密码\tClientID\tRefreshToken\t分组名
//
// 参数：
//   - text: 包含账号信息的多行文本
//
// 返回值：
//   - int: 成功导入的账号数量
//   - error: 始终返回nil（错误在内部处理）
func (s *AccountService) Import(text string) (int, error) {
	// 调用解析工具解析文本
	accounts, groupNames, _ := utils.ParseAccountsText(text)
	count := 0
	for i, acc := range accounts {
		// 确保分组存在（不存在则创建）
		groupID := s.ensureGroup(groupNames[i])
		// INSERT OR REPLACE：邮箱唯一，存在则更新
		_, err := database.DB.Exec(`INSERT OR REPLACE INTO accounts
			(email, password, client_id, refresh_token, group_id, status, updated_at)
			VALUES (?, ?, ?, ?, ?, 'active', CURRENT_TIMESTAMP)`,
			acc.Email, acc.Password, acc.ClientID, acc.RefreshToken, groupID)
		if err == nil {
			count++
		}
	}
	return count, nil
}

// ensureGroup 确保分组存在
//
// 内部方法，用于导入账号时自动创建分组
// 如果分组已存在则返回其ID，否则创建新分组
//
// 参数：
//   - name: 分组名称
//
// 返回值：
//   - int64: 分组ID
func (s *AccountService) ensureGroup(name string) int64 {
	var id int64
	// 先查询是否存在
	err := database.DB.QueryRow("SELECT id FROM groups WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id // 已存在，返回ID
	}
	// 不存在，创建新分组
	res, _ := database.DB.Exec("INSERT INTO groups (name) VALUES (?)", name)
	id, _ = res.LastInsertId()
	return id
}

// Delete 删除账号
//
// 参数：
//   - id: 要删除的账号ID
//
// 返回值：
//   - error: 删除失败时返回错误
func (s *AccountService) Delete(id int64) error {
	_, err := database.DB.Exec("DELETE FROM accounts WHERE id = ?", id)
	return err
}

// UpdateToken 更新账号的Token信息
//
// Token刷新成功后调用，同时更新状态为active并清除错误信息
//
// 参数：
//   - id: 账号ID
//   - accessToken: 新的访问令牌
//   - refreshToken: 新的刷新令牌（可能会更新）
//   - expiresAt: Token过期时间
//
// 返回值：
//   - error: 更新失败时返回错误
func (s *AccountService) UpdateToken(id int64, accessToken, refreshToken string, expiresAt time.Time) error {
	_, err := database.DB.Exec(`UPDATE accounts SET access_token = ?, refresh_token = ?,
		token_expires_at = ?, status = 'active', last_error = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		accessToken, refreshToken, expiresAt.Format(time.RFC3339), id)
	return err
}

// UpdateStatus 更新账号状态
//
// 用于标记账号为error状态（Token刷新失败时）
//
// 参数：
//   - id: 账号ID
//   - status: 状态值（"active"或"error"）
//   - lastError: 错误信息（status为error时设置）
//
// 返回值：
//   - error: 更新失败时返回错误
func (s *AccountService) UpdateStatus(id int64, status, lastError string) error {
	_, err := database.DB.Exec(`UPDATE accounts SET status = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, lastError, id)
	return err
}

// UpdateGroup 更新账号所属分组
//
// 参数：
//   - accountID: 账号ID
//   - groupID: 目标分组ID
//
// 返回值：
//   - error: 更新失败时返回错误
func (s *AccountService) UpdateGroup(accountID, groupID int64) error {
	_, err := database.DB.Exec("UPDATE accounts SET group_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", groupID, accountID)
	return err
}

// Count 获取账号总数
//
// 返回值：
//   - int: 数据库中的账号总数
func (s *AccountService) Count() int {
	var count int
	database.DB.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&count)
	return count
}

// DeleteByGroup 删除指定分组下的所有账号
//
// 用于清空分组功能
//
// 参数：
//   - groupID: 分组ID
//
// 返回值：
//   - error: 删除失败时返回错误
func (s *AccountService) DeleteByGroup(groupID int64) error {
	_, err := database.DB.Exec("DELETE FROM accounts WHERE group_id = ?", groupID)
	return err
}

// UpdateProtocol 更新账号的邮件访问协议类型
//
// 当REST API访问失败并成功回退到IMAP协议时，调用此方法将账号标记为IMAP类型。
// 后续访问该账号时将直接使用IMAP协议，避免重复尝试REST API。
//
// 参数：
//   - id: 账号ID
//   - protocol: 协议类型，可选值：
//     - "o2": Outlook REST API（默认）
//     - "imap": IMAP协议（用于Hotmail等个人账户）
//
// 返回值：
//   - error: 数据库更新失败时返回错误
func (s *AccountService) UpdateProtocol(id int64, protocol string) error {
	_, err := database.DB.Exec("UPDATE accounts SET protocol = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", protocol, id)
	return err
}
