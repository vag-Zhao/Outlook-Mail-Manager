/**
 * @file tailwind.config.js
 * @description Tailwind CSS 配置文件
 *
 * 定义项目的设计系统：
 * - 内容扫描路径（用于Tree-shaking未使用的样式）
 * - 自定义主题颜色（基于HSL色彩空间）
 * - 自定义圆角尺寸
 */

/** @type {import('tailwindcss').Config} */
export default {
  // 内容扫描配置
  // Tailwind 会扫描这些文件中使用的类名，移除未使用的样式
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],

  theme: {
    extend: {
      // 自定义颜色系统
      // 使用 HSL 格式便于调整明暗和饱和度
      colors: {
        // 边框颜色 - 浅灰蓝色
        border: 'hsl(214.3 31.8% 91.4%)',
        // 背景色 - 纯白
        background: 'hsl(0 0% 100%)',
        // 前景色（文字）- 深蓝黑色
        foreground: 'hsl(222.2 84% 4.9%)',
        // 主色调 - 蓝色系
        primary: {
          DEFAULT: 'hsl(221.2 83.2% 53.3%)',  // 主蓝色
          foreground: 'hsl(210 40% 98%)',      // 主色上的文字（浅色）
        },
        // 静音色 - 用于次要元素
        muted: {
          DEFAULT: 'hsl(210 40% 96.1%)',       // 浅灰背景
          foreground: 'hsl(215.4 16.3% 46.9%)', // 灰色文字
        },
        // 强调色 - 用于悬停等交互状态
        accent: {
          DEFAULT: 'hsl(210 40% 96.1%)',       // 浅灰背景
          foreground: 'hsl(222.2 47.4% 11.2%)', // 深色文字
        },
        // 危险色 - 用于删除、错误等
        destructive: {
          DEFAULT: 'hsl(0 84.2% 60.2%)',       // 红色
          foreground: 'hsl(210 40% 98%)',      // 红色上的文字（浅色）
        },
      },

      // 自定义圆角尺寸
      borderRadius: {
        lg: '0.5rem',   // 大圆角 8px
        md: '0.375rem', // 中圆角 6px
        sm: '0.25rem',  // 小圆角 4px
      },
    },
  },

  // 插件列表（当前未使用额外插件）
  plugins: [],
}
