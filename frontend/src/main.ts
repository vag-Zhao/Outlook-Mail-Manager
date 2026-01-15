/**
 * @file Vue应用入口
 * @description 创建Vue实例，挂载Pinia状态管理和根组件
 */
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './style.css'

// 创建Vue应用 -> 注册Pinia -> 挂载到#app
createApp(App).use(createPinia()).mount('#app')
