/**
 * @file postcss.config.js
 * @description PostCSS 配置文件
 *
 * PostCSS 是一个用 JavaScript 转换 CSS 的工具。
 * 本配置启用以下插件：
 * - tailwindcss: Tailwind CSS 框架，提供原子化CSS类
 * - autoprefixer: 自动添加浏览器厂商前缀，确保CSS兼容性
 */
export default {
  plugins: {
    // Tailwind CSS 插件
    // 处理 @tailwind 指令，生成工具类
    tailwindcss: {},

    // Autoprefixer 插件
    // 根据 browserslist 配置自动添加 -webkit-、-moz- 等前缀
    autoprefixer: {},
  },
}
