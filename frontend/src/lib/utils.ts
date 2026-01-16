/**
 * @file utils.ts
 * @description 前端通用工具函数库
 *
 * 提供项目中常用的工具函数：
 * - CSS类名合并工具（支持Tailwind CSS）
 * - 日期格式化工具（中文友好显示）
 */

import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * 合并CSS类名工具函数
 *
 * 结合clsx和tailwind-merge的功能：
 * - clsx: 条件性地合并类名，支持对象、数组等多种输入格式
 * - twMerge: 智能合并Tailwind CSS类名，解决类名冲突问题
 *
 * @param inputs - 可变参数，支持字符串、对象、数组等ClassValue类型
 * @returns 合并后的类名字符串
 *
 * @example
 * // 基础用法
 * cn('px-2', 'py-1') // => 'px-2 py-1'
 *
 * // 条件类名
 * cn('base', isActive && 'active') // => 'base active' 或 'base'
 *
 * // Tailwind冲突解决
 * cn('px-2', 'px-4') // => 'px-4' (后者覆盖前者)
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * 日期格式化工具函数
 *
 * 将日期字符串转换为中文友好的相对时间显示：
 * - 今天: 显示具体时间 (如 "14:30")
 * - 昨天: 显示 "昨天"
 * - 7天内: 显示 "X天前"
 * - 更早: 显示月日 (如 "1月15日")
 *
 * @param dateStr - ISO 8601格式的日期字符串
 * @returns 格式化后的中文日期字符串
 *
 * @example
 * formatDate('2026-01-16T14:30:00') // 今天 => '14:30'
 * formatDate('2026-01-15T10:00:00') // 昨天 => '昨天'
 * formatDate('2026-01-13T10:00:00') // 3天前 => '3天前'
 * formatDate('2026-01-01T10:00:00') // 更早 => '1月1日'
 */
export function formatDate(dateStr: string): string {
  // 解析输入的日期字符串
  const date = new Date(dateStr)
  // 获取当前时间用于计算时间差
  const now = new Date()
  // 计算时间差（毫秒）
  const diff = now.getTime() - date.getTime()
  // 将毫秒转换为天数
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))

  // 根据时间差返回不同格式
  if (days === 0) {
    // 今天：显示时:分格式
    return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  }
  if (days === 1) {
    // 昨天
    return '昨天'
  }
  if (days < 7) {
    // 一周内：显示X天前
    return `${days}天前`
  }
  // 更早：显示月日格式
  return date.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })
}
