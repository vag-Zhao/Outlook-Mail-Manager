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
	"os"
	"outlook-mail-manager/internal/database"
	"outlook-mail-manager/internal/models"
	"outlook-mail-manager/internal/services"
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
	tokenMu    sync.RWMutex              // Token缓存读写锁，保证并发安全
	tokens     map[int64]*tokenCache     // Token缓存映射表，key为账号ID
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
		tokens:     make(map[int64]*tokenCache),  // 初始化空的Token缓存
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
// 调用Microsoft Outlook API获取用户的邮件文件夹（收件箱、已发送、草稿等）
// 如果Token过期（返回401），会自动刷新Token并重试一次
//
// 参数：
//   - accountID: 账号ID
//
// 返回值：
//   - []models.MailFolder: 文件夹列表，包含ID、名称、未读数等
//   - error: API调用错误或Token刷新失败
func (a *App) GetMailFolders(accountID int64) ([]models.MailFolder, error) {
	// 获取有效的访问令牌
	token, err := a.ensureValidToken(accountID)
	if err != nil {
		return nil, err
	}
	// 调用Graph API获取文件夹
	result, err := a.graphSvc.GetMailFolders(token)
	// Token过期时自动重试：清除缓存，强制刷新Token，重新请求
	if err != nil && strings.Contains(err.Error(), "unauthorized") {
		a.clearTokenCache(accountID)
		token, err = a.getToken(accountID, true)
		if err != nil {
			return nil, err
		}
		return a.graphSvc.GetMailFolders(token)
	}
	return result, err
}

// GetMessages 获取指定文件夹的邮件列表
//
// 支持分页加载，每页20条邮件，按接收时间倒序排列
//
// 参数：
//   - accountID: 账号ID
//   - folderID: 文件夹ID（如"inbox"、"junkemail"）
//   - page: 页码，从0开始
//
// 返回值：
//   - []models.Message: 邮件列表
//   - error: API调用错误
func (a *App) GetMessages(accountID int64, folderID string, page int) ([]models.Message, error) {
	token, err := a.ensureValidToken(accountID)
	if err != nil {
		return nil, err
	}
	// 计算分页参数：skip = page * 20, top = 20
	result, err := a.graphSvc.GetMessages(token, folderID, page*20, 20)
	if err != nil && strings.Contains(err.Error(), "unauthorized") {
		a.clearTokenCache(accountID)
		token, err = a.getToken(accountID, true)
		if err != nil {
			return nil, err
		}
		return a.graphSvc.GetMessages(token, folderID, page*20, 20)
	}
	return result, err
}

// GetMessageDetail 获取邮件详情
//
// 获取邮件的完整内容，包括HTML正文
// 支持Token过期自动重试
//
// 参数：
//   - accountID: 账号ID
//   - messageID: 邮件ID
//
// 返回值：
//   - *models.Message: 邮件详情，包含完整正文
//   - error: API调用错误
func (a *App) GetMessageDetail(accountID int64, messageID string) (*models.Message, error) {
	token, err := a.ensureValidToken(accountID)
	if err != nil {
		return nil, err
	}
	result, err := a.graphSvc.GetMessage(token, messageID)
	if err != nil && strings.Contains(err.Error(), "unauthorized") {
		a.clearTokenCache(accountID)
		token, err = a.getToken(accountID, true)
		if err != nil {
			return nil, err
		}
		return a.graphSvc.GetMessage(token, messageID)
	}
	return result, err
}

// GetAttachments 获取邮件附件列表
//
// 获取指定邮件的所有附件信息
// 支持Token过期自动重试
//
// 参数：
//   - accountID: 账号ID
//   - messageID: 邮件ID
//
// 返回值：
//   - []models.Attachment: 附件列表，包含文件名、大小、Base64内容
//   - error: API调用错误
func (a *App) GetAttachments(accountID int64, messageID string) ([]models.Attachment, error) {
	token, err := a.ensureValidToken(accountID)
	if err != nil {
		return nil, err
	}
	result, err := a.graphSvc.GetAttachments(token, messageID)
	if err != nil && strings.Contains(err.Error(), "unauthorized") {
		a.clearTokenCache(accountID)
		token, err = a.getToken(accountID, true)
		if err != nil {
			return nil, err
		}
		return a.graphSvc.GetAttachments(token, messageID)
	}
	return result, err
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
	delete(a.tokens, accountID) // 从map中删除缓存项
	a.tokenMu.Unlock()
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
