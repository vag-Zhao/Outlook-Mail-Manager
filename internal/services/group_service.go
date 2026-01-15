// Package services 业务服务层
//
// group_service.go 分组服务
//
// 功能说明：
// - 分组的CRUD操作（增删改查）
// - 分组内账号数量统计
// - 删除分组时自动迁移账号到默认分组
package services

import (
	"database/sql"
	"outlook-mail-manager/internal/database"
	"outlook-mail-manager/internal/models"
)

// GroupService 分组服务
//
// 提供分组相关的所有数据库操作
type GroupService struct{}

// NewGroupService 创建分组服务实例
//
// 返回值：
//   - *GroupService: 服务实例
func NewGroupService() *GroupService {
	return &GroupService{}
}

// List 获取所有分组列表
//
// 使用子查询统计每个分组内的账号数量
// 按排序顺序和ID排序
//
// 返回值：
//   - []models.Group: 分组列表，包含账号数量
//   - error: 数据库查询错误
func (s *GroupService) List() ([]models.Group, error) {
	// SQL查询：获取分组信息并统计每个分组的账号数
	// 子查询 (SELECT COUNT(*) FROM accounts WHERE group_id = g.id) 计算账号数
	rows, err := database.DB.Query(`SELECT g.id, g.name, g.parent_id, g.sort_order,
		(SELECT COUNT(*) FROM accounts WHERE group_id = g.id) as count
		FROM groups g ORDER BY g.sort_order, g.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 遍历结果集构建分组列表
	var groups []models.Group
	for rows.Next() {
		var g models.Group
		var parentID sql.NullInt64 // 父分组ID可能为NULL
		err := rows.Scan(&g.ID, &g.Name, &parentID, &g.SortOrder, &g.Count)
		if err != nil {
			continue // 跳过解析失败的行
		}
		// 处理可空的父分组ID
		if parentID.Valid {
			g.ParentID = &parentID.Int64
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// Create 创建新分组
//
// 参数：
//   - name: 分组名称
//   - parentID: 父分组ID（可为nil，暂未使用嵌套功能）
//
// 返回值：
//   - *models.Group: 创建成功的分组对象
//   - error: 创建失败时返回错误
func (s *GroupService) Create(name string, parentID *int64) (*models.Group, error) {
	res, err := database.DB.Exec("INSERT INTO groups (name, parent_id) VALUES (?, ?)", name, parentID)
	if err != nil {
		return nil, err
	}
	// 获取自增ID
	id, _ := res.LastInsertId()
	return &models.Group{ID: id, Name: name, ParentID: parentID}, nil
}

// Update 更新分组名称
//
// 参数：
//   - id: 分组ID
//   - name: 新的分组名称
//
// 返回值：
//   - error: 更新失败时返回错误
func (s *GroupService) Update(id int64, name string) error {
	_, err := database.DB.Exec("UPDATE groups SET name = ? WHERE id = ?", name, id)
	return err
}

// Delete 删除分组
//
// 删除逻辑：
// 1. 先将该分组下的所有账号迁移到默认分组（ID=1）
// 2. 然后删除分组记录
// 3. 默认分组（ID=1）不能被删除
//
// 参数：
//   - id: 要删除的分组ID
//
// 返回值：
//   - error: 删除失败时返回错误
func (s *GroupService) Delete(id int64) error {
	// 先将该分组下的账号迁移到默认分组
	database.DB.Exec("UPDATE accounts SET group_id = 1 WHERE group_id = ?", id)
	// 删除分组（排除默认分组ID=1）
	_, err := database.DB.Exec("DELETE FROM groups WHERE id = ? AND id != 1", id)
	return err
}
