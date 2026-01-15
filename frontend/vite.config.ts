/**
 * @file Vite 构建配置
 * @description 配置Vue插件、路径别名和构建输出目录
 */
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src')  // 路径别名：@ -> src目录
    }
  },
  build: {
    outDir: 'dist',      // 构建输出目录
    emptyOutDir: true    // 构建前清空输出目录
  }
})
