// Package main 邮箱管家应用入口
//
// 本文件是Wails桌面应用的启动入口
//
// Wails框架说明：
// Wails是一个Go语言的桌面应用框架，允许使用Go作为后端、
// Web技术（HTML/CSS/JS）作为前端构建跨平台桌面应用
//
// 应用架构：
// - 前端：Vue3 + TypeScript + TailwindCSS
// - 后端：Go + SQLite
// - 通信：Wails自动生成的绑定，前端可直接调用Go方法
package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// assets 嵌入的前端静态资源
//
// 使用Go 1.16+的embed特性将前端构建产物嵌入到二进制文件中
// "all:frontend/dist" 表示嵌入frontend/dist目录下的所有文件
// 这样打包后的exe文件无需额外的资源文件即可运行
//
//go:embed all:frontend/dist
var assets embed.FS

// main 应用程序入口函数
//
// 执行流程：
// 1. 创建App实例（初始化所有服务）
// 2. 配置Wails运行时选项
// 3. 启动应用窗口
func main() {
	// 创建应用核心实例
	app := NewApp()

	// 启动Wails应用
	err := wails.Run(&options.App{
		Title:     "邮箱管家",  // 窗口标题
		Width:     1000,      // 初始窗口宽度（像素）
		Height:    600,       // 初始窗口高度（像素）
		MinWidth:  640,       // 最小窗口宽度
		MinHeight: 480,       // 最小窗口高度
		// 静态资源服务器配置
		AssetServer: &assetserver.Options{
			Assets: assets, // 使用嵌入的前端资源
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1}, // 窗口背景色（白色）
		OnStartup:        app.startup,                                  // 启动回调：初始化数据库
		OnShutdown:       app.shutdown,                                 // 关闭回调：清理资源
		// 绑定到前端的Go对象
		// 绑定后，前端可通过 window.go.main.App.MethodName() 调用
		Bind: []interface{}{
			app,
		},
		// Windows平台特定配置
		Windows: &windows.Options{
			WebviewIsTransparent: false, // WebView不透明
			WindowIsTranslucent:  false, // 窗口不透明
			DisableWindowIcon:    false, // 显示窗口图标
		},
	})

	// 启动失败时输出错误信息
	if err != nil {
		println("Error:", err.Error())
	}
}
