# 更新日志

## [1.2.0] - 2026-01-16 (第三版)

### 新增
- IMAP 服务器自动选择（个人账户用 `imap-mail.outlook.com`，企业账户用 `outlook.office365.com`）
- 底部状态栏加载中环状动画
- 协议更新事件监听（`protocol-updated`），实时更新账号协议标签
- 全面的调试日志（前端 console.log + 后端 log.Printf）

### 优化
- 协议检测策略重构：从简单的 `@hotmail.com` 后缀检测改为基于 `account.Protocol` 字段的智能判断
- REST API → IMAP 自动回退机制：O2 失败后自动尝试 IMAP 并标记账号协议
- TLS 连接配置增强：SNI 支持、30秒超时、强制 TLS 1.2+
- 账号列表显示协议类型（📧 IMAP / ☁️ O2）替代原来的状态显示（✓ 正常 / ✗ 异常）
- 文件夹数量显示逻辑优化（优先显示 totalItemCount）

### 修复
- 深色模式下分组右键菜单悬停颜色问题（文字不可见）
- 邮件详情加载动画边框样式（`border-3` → `border-4`，修复 Tailwind 兼容性）
- 切换账号时的请求中断处理，避免旧请求覆盖新数据

### 技术细节
- `imap_service.go`: 新增 `getIMAPServer()` 服务器选择函数
- `account_service.go`: 新增 `UpdateProtocol()` 方法用于标记账号协议类型
- `app.go`: 重构协议检测逻辑，添加 `getIMAPToken()` 缓存策略注释
- `App.vue`: 协议更新事件监听、深色模式适配、调试日志

## [1.1.0] - 2026-01-16 (第二版)

### 新增
- Hotmail 邮箱 IMAP 协议支持（XOAUTH2 认证）
- 账号级别缓存机制，切换账号瞬时响应
- 右键菜单"刷新邮件"功能
- 邮件详情加载 Loading 动画
- 点击账号自动进入收件箱
- IMAP 服务器自动选择（个人账户用 imap-mail.outlook.com，企业账户用 outlook.office365.com）
- 底部状态栏加载中环状动画

### 优化
- IMAP 性能优化：预编译正则表达式
- 仅查询收件箱和垃圾邮件的 STATUS（减少 75% IMAP 命令）
- 连接池复用（5分钟内复用同一连接）
- 前端文件夹数量显示优化（未读数/总数）
- TLS 连接配置优化（SNI 支持、30秒超时、TLS 1.2+）
- 代码注释全面优化，覆盖所有核心模块

### 修复
- Outlook Token 刷新 scope 问题（invalid_grant 错误）
- IMAP 文件夹名解析（支持无引号格式）
- 文件夹 ID 映射（REST API ID ↔ IMAP 名称）
- HTML 邮件脚本清理（消除 sandbox 警告）
- 清空分组后重置中间栏和右侧栏状态
- 深色模式下分组右键菜单颜色问题（文字不可见）
- 邮件详情加载动画边框样式（border-3 → border-4）

### 技术细节
- `imap_service.go`: IMAP 客户端实现，连接池，MIME 解析，服务器自动选择
- `token_service.go`: 双 scope 支持（REST API / IMAP），详细注释
- `app.go`: Hotmail 检测，统一 HTML 清理，getIMAPToken 缓存策略
- `mail.ts`: 账号缓存，detailLoading 状态
- `App.vue`: 右键刷新，Loading 效果，深色模式适配
- `utils.ts`: cn() 和 formatDate() 函数详细注释
- `tailwind.config.js`: 完整的颜色系统和配置注释

## [1.0.0] - 2026-01-15

### 初始版本
- 多账号管理（导入、分组、删除）
- Outlook 邮箱 REST API 支持
- 邮件列表和详情查看
- 附件下载
- 深色模式
- Token 自动刷新
