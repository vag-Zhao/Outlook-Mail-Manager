// Package database 数据库层
//
// sqlite.go SQLite数据库初始化和管理
//
// 功能说明：
// - 数据库连接初始化
// - 数据表结构迁移
// - 数据库连接关闭
//
// 数据库位置：~/.outlook-mail-manager/data.db
// 使用的驱动：github.com/mattn/go-sqlite3
package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3" // SQLite驱动，使用空白导入注册驱动
)

// DB 全局数据库连接实例
//
// 在应用启动时由Init()初始化
// 所有数据库操作都通过此实例进行
var DB *sql.DB

// Init 初始化数据库连接
//
// 执行流程：
// 1. 获取用户主目录
// 2. 创建应用数据目录（~/.outlook-mail-manager）
// 3. 打开SQLite数据库连接
// 4. 执行数据表迁移
//
// 返回值：
//   - error: 初始化失败时返回错误（目录创建失败、数据库连接失败等）
func Init() error {
	// 获取用户主目录（Windows: C:\Users\xxx, Linux/Mac: /home/xxx）
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}
	// 构建数据库目录路径
	dbDir := filepath.Join(homeDir, ".outlook-mail-manager")
	// 创建目录（如果不存在），权限755
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("create db dir: %w", err)
	}
	// 构建数据库文件路径
	dbPath := filepath.Join(dbDir, "data.db")

	// 打开SQLite数据库连接
	// 如果文件不存在会自动创建
	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	// 执行数据表迁移
	return migrate()
}

// migrate 执行数据库迁移
//
// 创建应用所需的数据表和索引
// 使用 IF NOT EXISTS 确保可重复执行（幂等性）
//
// 表结构说明：
//
// groups 分组表：
//   - id: 主键，自增
//   - name: 分组名称，不能为空
//   - parent_id: 父分组ID（预留，暂未使用）
//   - sort_order: 排序顺序
//   - created_at: 创建时间
//
// accounts 账号表：
//   - id: 主键，自增
//   - email: 邮箱地址，唯一约束
//   - password: 邮箱密码（可选）
//   - client_id: OAuth2客户端ID
//   - refresh_token: OAuth2刷新令牌
//   - access_token: OAuth2访问令牌（缓存）
//   - token_expires_at: 令牌过期时间
//   - group_id: 所属分组ID，外键关联groups表
//   - display_name: 显示名称
//   - status: 状态（active/error）
//   - last_error: 最后一次错误信息
//   - created_at: 创建时间
//   - updated_at: 更新时间
//
// 返回值：
//   - error: SQL执行错误
func migrate() error {
	schema := `
	-- 分组表：用于组织和管理账号
	CREATE TABLE IF NOT EXISTS groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		parent_id INTEGER,
		sort_order INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 账号表：存储Outlook邮箱账号信息
	CREATE TABLE IF NOT EXISTS accounts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL UNIQUE,
		password TEXT,
		client_id TEXT NOT NULL,
		refresh_token TEXT NOT NULL,
		access_token TEXT,
		token_expires_at DATETIME,
		group_id INTEGER,
		display_name TEXT,
		status TEXT DEFAULT 'active',
		last_error TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE SET NULL
	);

	-- 索引：加速邮箱查询（用于去重和查找）
	CREATE INDEX IF NOT EXISTS idx_accounts_email ON accounts(email);
	-- 索引：加速按分组筛选
	CREATE INDEX IF NOT EXISTS idx_accounts_group ON accounts(group_id);

	-- 初始化默认分组（ID=1）
	INSERT OR IGNORE INTO groups (id, name) VALUES (1, '默认分组');
	`
	// 执行SQL语句
	_, err := DB.Exec(schema)
	return err
}

// Close 关闭数据库连接
//
// 在应用退出时调用，释放数据库资源
// 安全检查：仅在DB不为nil时关闭
func Close() {
	if DB != nil {
		DB.Close()
	}
}
