# 贡献指南

感谢你对本项目的关注！欢迎提交 Issue 和 Pull Request。

## 开发环境

1. 安装 Go 1.21+
2. 安装 Node.js 18+
3. 安装 Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

## 开发流程

1. Fork 本仓库
2. 创建功能分支: `git checkout -b feature/your-feature`
3. 提交更改: `git commit -m 'Add some feature'`
4. 推送分支: `git push origin feature/your-feature`
5. 提交 Pull Request

## 代码规范

- Go 代码遵循官方规范，使用 `gofmt` 格式化
- 前端代码使用 ESLint + Prettier
- 提交信息使用中文或英文均可

## 运行开发环境

```bash
wails dev
```
