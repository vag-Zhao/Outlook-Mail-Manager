# 邮箱管家 (Outlook Mail Manager)

基于 Wails 构建的 Outlook / Hotmail 邮箱批量管理桌面应用，支持多账号管理、双协议访问、邮件查看等功能。

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Platform](https://img.shields.io/badge/platform-Windows-lightgrey.svg)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)
![Vue](https://img.shields.io/badge/Vue-3.x-4FC08D.svg)
![Version](https://img.shields.io/badge/version-1.2.0-green.svg)

## 截图预览

| 邮件视图 | 管理视图 | 深色模式 |
|:---:|:---:|:---:|
| ![邮件视图](screenshots/mail-view.png) | ![管理视图](screenshots/manage-view.png) | ![深色模式](screenshots/dark-mode.png) |

## 功能特性

### 多账号管理
- 批量导入 Outlook / Hotmail 账号（支持多种分隔格式）
- 分组管理：创建、删除、拖拽移动
- 批量操作：检测 Token 有效性、删除、移动分组
- 分组账号一键导出

### 邮件查看
- 文件夹浏览：收件箱、垃圾邮件、已发送等
- 邮件分页加载，支持加载更多
- HTML 邮件正文渲染（自动清理脚本，安全显示）
- 附件列表查看与下载
- 加载状态动画反馈

### 双协议智能切换
| 协议 | 适用场景 | 特点 |
|------|---------|------|
| REST API (O2) | Outlook 企业账户 | 快速、功能丰富 |
| IMAP + XOAUTH2 | Hotmail 个人账户 | 兼容性好，自动回退 |

- 自动检测账号类型，智能选择协议
- REST API 失败时自动回退到 IMAP
- IMAP 服务器自动选择（个人账户 / 企业账户）

### 性能优化
- **账号级缓存**：切换账号瞬时响应，无需重复加载
- **IMAP 连接池**：5分钟内复用同一连接，减少握手开销
- **预编译正则**：优化邮件解析性能
- **智能查询**：仅查询必要文件夹的 STATUS，减少 75% IMAP 命令

### 界面特性
- 双视图切换：邮件视图 / 管理视图
- 深色模式支持，一键切换
- 右键菜单：刷新邮件、复制邮箱、移动分组
- 协议类型实时显示（📧 IMAP / ☁️ O2）

## 技术栈

| 层级 | 技术 | 版本 |
|------|------|------|
| 桌面框架 | [Wails](https://wails.io/) | v2.11.0 |
| 后端 | Go | 1.21+ |
| 前端 | Vue + TypeScript | 3.x |
| 状态管理 | Pinia | 2.x |
| 样式 | Tailwind CSS | 3.x |
| 图标 | Lucide Icons | - |
| 数据库 | SQLite | - |
| 邮件 API | Microsoft Outlook REST API | v2.0 |
| 邮件协议 | IMAP + XOAUTH2 | - |

## 系统要求

- Windows 10 / 11
- WebView2 Runtime（Windows 11 已内置，Windows 10 需安装）

## 快速开始

### 下载安装

从 [Releases](https://github.com/user/outlook-mail-manager/releases) 页面下载最新版本，双击运行即可。

### 从源码构建

```bash
# 前置条件
# - Go 1.21+
# - Node.js 18+
# - Wails CLI

# 安装 Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 克隆仓库
git clone https://github.com/user/outlook-mail-manager.git
cd outlook-mail-manager

# 开发模式
wails dev

# 构建生产版本
wails build
```

构建产物位于 `build/bin/邮箱管家.exe`

## 使用说明

### 导入账号

点击左上角导入按钮，粘贴账号数据：

```
# 格式1：四横线分隔
邮箱----密码----ClientID----RefreshToken----分组名

# 格式2：Tab分隔
邮箱	密码	ClientID	RefreshToken	分组名
```

| 字段 | 必填 | 说明 |
|------|:---:|------|
| 邮箱 | ✓ | Outlook / Hotmail 邮箱地址 |
| 密码 | - | 邮箱密码（仅用于记录，不参与认证） |
| ClientID | ✓ | Azure AD 应用的 Client ID |
| RefreshToken | ✓ | OAuth2 Refresh Token |
| 分组名 | - | 自动创建分组（可选） |

### 获取 Token

1. 在 [Azure Portal](https://portal.azure.com/) 注册应用
2. 配置重定向 URI：`http://localhost`
3. 添加 API 权限：`Mail.Read`、`IMAP.AccessAsUser.All`、`offline_access`
4. 通过 OAuth2 授权流程获取 RefreshToken

## 项目结构

```
outlook-mail-manager/
├── app.go                      # 应用核心控制器（Wails 绑定）
├── main.go                     # 程序入口
├── internal/
│   ├── database/sqlite.go      # SQLite 数据库初始化
│   ├── models/                 # 数据模型
│   │   ├── account.go          # 账号、分组模型
│   │   └── mail.go             # 邮件、文件夹模型
│   ├── services/               # 业务服务层
│   │   ├── account_service.go  # 账号 CRUD
│   │   ├── group_service.go    # 分组 CRUD
│   │   ├── graph_service.go    # Outlook REST API
│   │   ├── imap_service.go     # IMAP 协议（Hotmail）
│   │   └── token_service.go    # OAuth2 Token 刷新
│   └── utils/parser.go         # 账号文本解析
└── frontend/
    ├── src/
    │   ├── App.vue             # 主组件（UI + 逻辑）
    │   ├── stores/             # Pinia 状态管理
    │   │   ├── account.ts      # 账号状态
    │   │   └── mail.ts         # 邮件状态
    │   └── lib/utils.ts        # 工具函数
    └── tailwind.config.js      # Tailwind 配置
```

## 安全说明

- 所有数据存储在本地 SQLite 数据库（`~/.outlook-mail-manager/data.db`）
- RefreshToken 等敏感信息仅存储在本地，不上传任何第三方服务器
- HTML 邮件自动清理 `<script>`、`on*` 事件、`javascript:` 等危险内容

## 更新日志

查看 [CHANGELOG.md](CHANGELOG.md) 了解版本更新历史。

## 开源协议

[MIT License](LICENSE)

## 作者

**ZGS** - [zgs3344@hunnu.edu.cn](mailto:zgs3344@hunnu.edu.cn)

## 致谢

- [Wails](https://wails.io/) - Go + Web 桌面应用框架
- [Vue.js](https://vuejs.org/) - 渐进式 JavaScript 框架
- [Tailwind CSS](https://tailwindcss.com/) - 实用优先的 CSS 框架
- [Lucide](https://lucide.dev/) - 精美的开源图标库

---

如果这个项目对你有帮助，欢迎 ⭐ Star 支持！
