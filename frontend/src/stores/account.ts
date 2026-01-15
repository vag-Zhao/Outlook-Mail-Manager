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
    if (!selectedGroupId.value) return accounts.value
    return accounts.value.filter(a => a.groupId === selectedGroupId.value)
  })

  // ============================================================================
  // 账号操作
  // ============================================================================

  /** 加载所有账号 */
  async function loadAccounts() {
    loading.value = true
    try {
      // @ts-ignore
      accounts.value = await window.go.main.App.GetAccounts(null) || []
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
    // @ts-ignore
    const count = await window.go.main.App.ImportAccounts(content)
    await loadAccounts()
    await loadGroups()
    return count
  }

  /**
   * 删除账号
   * @param id - 账号ID
   */
  async function deleteAccount(id: number) {
    // @ts-ignore
    await window.go.main.App.DeleteAccount(id)
    await loadAccounts()
  }

  // ============================================================================
  // 分组操作
  // ============================================================================

  /** 加载所有分组 */
  async function loadGroups() {
    // @ts-ignore
    groups.value = await window.go.main.App.GetGroups() || []
  }

  /**
   * 创建新分组
   * @param name - 分组名称
   */
  async function createGroup(name: string) {
    // @ts-ignore
    await window.go.main.App.CreateGroup(name)
    await loadGroups()
  }

  /**
   * 删除分组（分组内账号会移至未分组）
   * @param id - 分组ID
   */
  async function deleteGroup(id: number) {
    // @ts-ignore
    await window.go.main.App.DeleteGroup(id)
    await loadGroups()
    await loadAccounts()
  }

  /**
   * 移动账号到指定分组
   * @param accountId - 账号ID
   * @param groupId - 目标分组ID
   */
  async function moveToGroup(accountId: number, groupId: number) {
    // @ts-ignore
    await window.go.main.App.MoveAccountToGroup(accountId, groupId)
    await loadAccounts()
    await loadGroups()
  }

  /**
   * 清空分组内所有账号
   * @param groupId - 分组ID
   */
  async function clearGroup(groupId: number) {
    // @ts-ignore
    await window.go.main.App.ClearGroup(groupId)
    await loadAccounts()
    await loadGroups()
  }

  return {
    accounts, groups, selectedAccountId, selectedGroupId, loading,
    filteredAccounts,
    loadAccounts, loadGroups, importAccounts, deleteAccount, createGroup, deleteGroup, moveToGroup, clearGroup
  }
})
