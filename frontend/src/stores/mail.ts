/**
 * @file 邮件状态管理 Store
 * @description 管理邮件文件夹、邮件列表、邮件详情和附件的状态
 */
import { defineStore } from 'pinia'
import { ref } from 'vue'

/** 邮件文件夹接口 */
interface MailFolder {
  id: string
  displayName: string
  totalItemCount: number
  unreadItemCount: number
}

/** 邮件消息接口 */
interface Message {
  id: string
  subject: string
  bodyPreview: string
  body?: { contentType: string; content: string }
  from?: { emailAddress: { name: string; address: string } }
  receivedDateTime: string
  hasAttachments: boolean
  isRead: boolean
}

/** 附件接口 */
interface Attachment {
  id: string
  name: string
  contentType: string
  size: number
  contentBytes?: string  // Base64编码的附件内容
}

export const useMailStore = defineStore('mail', () => {
  // 英文文件夹名到中文的映射
  const folderNameMap: Record<string, string> = {
    'Inbox': '收件箱',
    'Sent Items': '已发送',
    'Drafts': '草稿',
    'Deleted Items': '已删除',
    'Junk Email': '垃圾邮件',
    'Archive': '存档',
    'Outbox': '发件箱',
    'Conversation History': '对话历史',
    'Notes': '便笺',
  }

  // 默认显示的文件夹列表
  const defaultFolders: MailFolder[] = [
    { id: 'inbox', displayName: '收件箱', totalItemCount: 0, unreadItemCount: 0 },
    { id: 'junkemail', displayName: '垃圾邮件', totalItemCount: 0, unreadItemCount: 0 },
  ]

  // ============================================================================
  // 响应式状态
  // ============================================================================
  const folders = ref<MailFolder[]>([...defaultFolders])
  const messages = ref<Message[]>([])
  const currentMessage = ref<Message | null>(null)
  const attachments = ref<Attachment[]>([])
  const selectedFolderId = ref<string | null>(null)
  const loading = ref(false)
  const detailLoading = ref(false)
  const currentPage = ref(0)
  const error = ref<string | null>(null)

  // 请求版本号，用于中断旧请求
  let requestId = 0
  // 当前正在加载的账号ID
  let currentAccountId: number | null = null

  // 邮件详情缓存，避免重复请求
  const messageCache = new Map<string, { message: Message; attachments: Attachment[] }>()

  // 账号级别缓存（一次会话内有效）
  const accountCache = new Map<number, {
    folders: MailFolder[]
    messages: Map<string, Message[]>  // folderId -> messages
  }>()

  // ============================================================================
  // 文件夹操作
  // ============================================================================

  /**
   * 加载账号的邮件文件夹列表
   * @param accountId - 账号ID
   * @param forceRefresh - 是否强制刷新
   */
  async function loadFolders(accountId: number, forceRefresh = false): Promise<boolean> {
    console.log('[MailStore] loadFolders 开始 - accountId:', accountId, 'forceRefresh:', forceRefresh)

    const myRequestId = ++requestId  // 总是递增，中断之前的请求
    currentAccountId = accountId     // 记录当前账号

    // 检查缓存
    if (!forceRefresh && accountCache.has(accountId)) {
      const cached = accountCache.get(accountId)!
      console.log('[MailStore] loadFolders 使用缓存 - 文件夹数量:', cached.folders.length)
      folders.value = cached.folders
      // 清空邮件数据，等待 loadMessages 加载
      messages.value = []
      currentMessage.value = null
      attachments.value = []
      return true
    }
    // 立即清空旧数据，避免显示错乱
    folders.value = [...defaultFolders]
    messages.value = []
    currentMessage.value = null
    attachments.value = []
    loading.value = true
    error.value = null
    try {
      console.log('[MailStore] loadFolders 调用后端 GetMailFolders...')
      // @ts-ignore
      const apiFolders = await window.go.main.App.GetMailFolders(accountId) || []

      // 检查是否已被新请求取代或账号已切换
      if (myRequestId !== requestId || currentAccountId !== accountId) {
        console.log('[MailStore] loadFolders 请求已过期，丢弃结果')
        return false
      }

      console.log('[MailStore] loadFolders 后端返回 - 文件夹数量:', apiFolders.length)
      apiFolders.forEach((f: MailFolder, i: number) => {
        console.log(`[MailStore] 文件夹[${i}]: id=${f.id}, name=${f.displayName}, total=${f.totalItemCount}, unread=${f.unreadItemCount}`)
      })

      // 只更新默认文件夹的数量，保持列表结构不变
      const newFolders = defaultFolders.map(df => {
        const match = apiFolders.find((f: MailFolder) =>
          f.id.toLowerCase() === df.id.toLowerCase() ||
          f.displayName === df.displayName ||
          folderNameMap[f.displayName] === df.displayName
        )
        if (match) {
          console.log(`[MailStore] 匹配文件夹: ${df.id} -> total=${match.totalItemCount}, unread=${match.unreadItemCount}`)
        }
        return match ? { ...df, totalItemCount: match.totalItemCount, unreadItemCount: match.unreadItemCount } : df
      })
      folders.value = newFolders
      console.log('[MailStore] loadFolders 完成 - 最终文件夹:', newFolders)

      // 更新缓存
      if (!accountCache.has(accountId)) {
        accountCache.set(accountId, { folders: newFolders, messages: new Map() })
      } else {
        accountCache.get(accountId)!.folders = newFolders
      }
      return true
    } catch (e: any) {
      console.error('[MailStore] loadFolders 失败:', e)
      error.value = String(e)
      return false
    } finally {
      loading.value = false
    }
  }

  // ============================================================================
  // 邮件列表操作
  // ============================================================================

  /**
   * 加载指定文件夹的邮件列表（支持分页）
   * @param accountId - 账号ID
   * @param folderId - 文件夹ID
   * @param page - 页码，0为首页
   * @param forceRefresh - 是否强制刷新
   */
  async function loadMessages(accountId: number, folderId: string, page = 0, forceRefresh = false) {
    console.log('[MailStore] loadMessages 开始 - accountId:', accountId, 'folderId:', folderId, 'page:', page, 'forceRefresh:', forceRefresh)

    const myRequestId = ++requestId  // 总是递增，中断之前的请求

    // 检查账号是否已切换
    if (currentAccountId !== accountId) {
      console.log('[MailStore] loadMessages 账号已切换，取消加载')
      return
    }

    // 检查缓存（仅首页）
    if (!forceRefresh && page === 0 && accountCache.has(accountId)) {
      const cached = accountCache.get(accountId)!
      if (cached.messages.has(folderId)) {
        const cachedMsgs = cached.messages.get(folderId)!
        console.log('[MailStore] loadMessages 使用缓存 - 邮件数量:', cachedMsgs.length)
        messages.value = cachedMsgs
        currentPage.value = 0
        return
      }
    }

    // 首页请求时立即清空旧数据
    if (page === 0) {
      messages.value = []
      currentMessage.value = null
      attachments.value = []
    }
    loading.value = true
    try {
      console.log('[MailStore] loadMessages 调用后端 GetMessages...')
      // @ts-ignore
      const msgs = await window.go.main.App.GetMessages(accountId, folderId, page) || []

      // 检查是否已被新请求取代或账号已切换
      if (myRequestId !== requestId || currentAccountId !== accountId) {
        console.log('[MailStore] loadMessages 请求已过期，丢弃结果')
        return
      }

      console.log('[MailStore] loadMessages 后端返回 - 邮件数量:', msgs.length)
      msgs.forEach((m: Message, i: number) => {
        console.log(`[MailStore] 邮件[${i}]: id=${m.id}, subject=${m.subject}, isRead=${m.isRead}`)
      })

      // 首页替换，后续页追加
      if (page === 0) {
        messages.value = msgs
        // 缓存首页数据
        if (accountCache.has(accountId)) {
          accountCache.get(accountId)!.messages.set(folderId, msgs)
        }
      } else {
        messages.value = [...messages.value, ...msgs]
        console.log('[MailStore] loadMessages 追加后总数:', messages.value.length)
      }
      currentPage.value = page
    } catch (e: any) {
      console.error('[MailStore] loadMessages 失败:', e)
    } finally {
      loading.value = false
    }
  }

  // ============================================================================
  // 邮件详情操作
  // ============================================================================

  /**
   * 加载邮件详情和附件（带缓存）
   * @param accountId - 账号ID
   * @param messageId - 邮件ID
   * @param folderId - 文件夹ID
   */
  async function loadMessageDetail(accountId: number, messageId: string, folderId?: string) {
    console.log('[MailStore] loadMessageDetail 开始 - accountId:', accountId, 'messageId:', messageId, 'folderId:', folderId)

    const myRequestId = ++requestId  // 总是递增，中断之前的请求

    const cacheKey = `${accountId}-${messageId}`
    const cached = messageCache.get(cacheKey)
    if (cached) {
      console.log('[MailStore] loadMessageDetail 使用缓存')
      currentMessage.value = cached.message
      attachments.value = cached.attachments
      return
    }

    detailLoading.value = true
    try {
      console.log('[MailStore] loadMessageDetail 调用后端 GetMessageDetail 和 GetAttachments...')
      // 并行加载邮件详情和附件
      // @ts-ignore
      const [msg, atts] = await Promise.all([
        window.go.main.App.GetMessageDetail(accountId, messageId, folderId || selectedFolderId.value || 'inbox'),
        window.go.main.App.GetAttachments(accountId, messageId)
      ])

      // 检查是否已被新请求取代或账号已切换
      if (myRequestId !== requestId || currentAccountId !== accountId) {
        console.log('[MailStore] loadMessageDetail 请求已过期，丢弃结果')
        return
      }

      console.log('[MailStore] loadMessageDetail 后端返回 - msg.id:', msg?.id, 'msg.subject:', msg?.subject)
      console.log('[MailStore] loadMessageDetail 附件数量:', atts?.length || 0)

      currentMessage.value = msg
      attachments.value = atts || []
      messageCache.set(cacheKey, { message: msg, attachments: atts || [] })
      console.log('[MailStore] loadMessageDetail 完成并缓存')
    } catch (e: any) {
      console.error('[MailStore] loadMessageDetail 失败:', e)
    } finally {
      detailLoading.value = false
    }
  }

  /**
   * 重置所有邮件状态
   */
  function reset() {
    console.log('[MailStore] reset 重置所有状态')
    folders.value = [...defaultFolders]
    messages.value = []
    currentMessage.value = null
    attachments.value = []
    selectedFolderId.value = null
    currentPage.value = 0
    error.value = null
  }

  /**
   * 清除指定账号的缓存（用于手动刷新）
   * @param accountId - 账号ID
   */
  function clearAccountCache(accountId: number) {
    console.log('[MailStore] clearAccountCache - accountId:', accountId)
    accountCache.delete(accountId)
    // 清除该账号的邮件详情缓存
    let cleared = 0
    for (const key of messageCache.keys()) {
      if (key.startsWith(`${accountId}-`)) {
        messageCache.delete(key)
        cleared++
      }
    }
    console.log('[MailStore] clearAccountCache 清除邮件详情缓存数量:', cleared)
  }

  return {
    folders, messages, currentMessage, attachments, selectedFolderId,
    loading, detailLoading, currentPage, error,
    loadFolders, loadMessages, loadMessageDetail, reset, clearAccountCache
  }
})
