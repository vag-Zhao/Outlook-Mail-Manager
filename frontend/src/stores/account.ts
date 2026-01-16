/**
 * @file 账号状态管理 Store
 * @description 管理邮箱账号和分组的CRUD操作
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

/** 账号接口 */
interface Account {
  id: number
  email: string
  password?: string
  clientId: string
  refreshToken?: string
  groupId?: number
  groupName?: string
  displayName?: string
  status: string       // 账号状态：active/invalid等
  protocol?: string    // 协议类型：o2/imap
  lastError?: string   // 最后一次错误信息
}

/** 分组接口 */
interface Group {
  id: number
  name: string
  parentId?: number
  count?: number  // 分组内账号数量
}

export const useAccountStore = defineStore('account', () => {
  // ============================================================================
  // 响应式状态
  // ============================================================================
  const accounts = ref<Account[]>([])
  const groups = ref<Group[]>([])
  const selectedAccountId = ref<number | null>(null)
  const selectedGroupId = ref<number | null>(null)
  const loading = ref(false)

  // ============================================================================
  // 计算属性
  // ============================================================================

  /** 根据选中分组过滤账号列表 */
  const filteredAccounts = computed(() => {
    const result = !selectedGroupId.value
      ? accounts.value
      : accounts.value.filter(a => a.groupId === selectedGroupId.value)
    console.log('[AccountStore] filteredAccounts 计算 - groupId:', selectedGroupId.value, '结果数量:', result.length)
    return result
  })

  // ============================================================================
  // 账号操作
  // ============================================================================

  /** 加载所有账号 */
  async function loadAccounts() {
    console.log('[AccountStore] loadAccounts 开始')
    loading.value = true
    try {
      // @ts-ignore
      accounts.value = await window.go.main.App.GetAccounts(null) || []
      console.log('[AccountStore] loadAccounts 成功 - 账号数量:', accounts.value.length)
      accounts.value.forEach((a, i) => {
        console.log(`[AccountStore] 账号[${i}]: id=${a.id}, email=${a.email}, protocol=${a.protocol}, status=${a.status}`)
      })
    } catch (e) {
      console.error('[AccountStore] loadAccounts 失败:', e)
    } finally {
      loading.value = false
    }
  }

  /**
   * 导入账号（从文本解析）
   * @param content - 账号文本内容
   * @returns 导入成功的账号数量
   */
  async function importAccounts(content: string): Promise<number> {
    console.log('[AccountStore] importAccounts 开始 - 内容长度:', content.length)
    try {
      // @ts-ignore
      const count = await window.go.main.App.ImportAccounts(content)
      console.log('[AccountStore] importAccounts 成功 - 导入数量:', count)
      await loadAccounts()
      await loadGroups()
      return count
    } catch (e) {
      console.error('[AccountStore] importAccounts 失败:', e)
      throw e
    }
  }

  /**
   * 删除账号
   * @param id - 账号ID
   */
  async function deleteAccount(id: number) {
    console.log('[AccountStore] deleteAccount 开始 - id:', id)
    try {
      // @ts-ignore
      await window.go.main.App.DeleteAccount(id)
      console.log('[AccountStore] deleteAccount 成功')
      await loadAccounts()
      await loadGroups() // 更新分组计数
    } catch (e) {
      console.error('[AccountStore] deleteAccount 失败:', e)
      throw e
    }
  }

  // ============================================================================
  // 分组操作
  // ============================================================================

  /** 加载所有分组 */
  async function loadGroups() {
    console.log('[AccountStore] loadGroups 开始')
    try {
      // @ts-ignore
      groups.value = await window.go.main.App.GetGroups() || []
      console.log('[AccountStore] loadGroups 成功 - 分组数量:', groups.value.length)
      groups.value.forEach((g, i) => {
        console.log(`[AccountStore] 分组[${i}]: id=${g.id}, name=${g.name}, count=${g.count}`)
      })
    } catch (e) {
      console.error('[AccountStore] loadGroups 失败:', e)
    }
  }

  /**
   * 创建新分组
   * @param name - 分组名称
   */
  async function createGroup(name: string) {
    console.log('[AccountStore] createGroup 开始 - name:', name)
    try {
      // @ts-ignore
      await window.go.main.App.CreateGroup(name)
      console.log('[AccountStore] createGroup 成功')
      await loadGroups()
    } catch (e) {
      console.error('[AccountStore] createGroup 失败:', e)
      throw e
    }
  }

  /**
   * 删除分组（分组内账号会移至未分组）
   * @param id - 分组ID
   */
  async function deleteGroup(id: number) {
    console.log('[AccountStore] deleteGroup 开始 - id:', id)
    try {
      // @ts-ignore
      await window.go.main.App.DeleteGroup(id)
      console.log('[AccountStore] deleteGroup 成功')
      await loadGroups()
      await loadAccounts()
    } catch (e) {
      console.error('[AccountStore] deleteGroup 失败:', e)
      throw e
    }
  }

  /**
   * 移动账号到指定分组
   * @param accountId - 账号ID
   * @param groupId - 目标分组ID
   */
  async function moveToGroup(accountId: number, groupId: number) {
    console.log('[AccountStore] moveToGroup 开始 - accountId:', accountId, 'groupId:', groupId)
    try {
      // @ts-ignore
      await window.go.main.App.MoveAccountToGroup(accountId, groupId)
      console.log('[AccountStore] moveToGroup 成功')
      await loadAccounts()
      await loadGroups()
    } catch (e) {
      console.error('[AccountStore] moveToGroup 失败:', e)
      throw e
    }
  }

  /**
   * 清空分组内所有账号
   * @param groupId - 分组ID
   */
  async function clearGroup(groupId: number) {
    console.log('[AccountStore] clearGroup 开始 - groupId:', groupId)
    try {
      // @ts-ignore
      await window.go.main.App.ClearGroup(groupId)
      console.log('[AccountStore] clearGroup 成功')
      await loadAccounts()
      await loadGroups()
    } catch (e) {
      console.error('[AccountStore] clearGroup 失败:', e)
      throw e
    }
  }

  return {
    accounts, groups, selectedAccountId, selectedGroupId, loading,
    filteredAccounts,
    loadAccounts, loadGroups, importAccounts, deleteAccount, createGroup, deleteGroup, moveToGroup, clearGroup
  }
})
