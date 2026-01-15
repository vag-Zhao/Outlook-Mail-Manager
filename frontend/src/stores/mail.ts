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
  const currentPage = ref(0)
  const error = ref<string | null>(null)

  // 邮件详情缓存，避免重复请求
  const messageCache = new Map<string, { message: Message; attachments: Attachment[] }>()

  // ============================================================================
  // 文件夹操作
  // ============================================================================

  /**
   * 加载账号的邮件文件夹列表
   * @param accountId - 账号ID
   */
  async function loadFolders(accountId: number) {
    loading.value = true
    error.value = null
    try {
      // @ts-ignore
      const apiFolders = await window.go.main.App.GetMailFolders(accountId) || []
      // 只更新默认文件夹的数量，保持列表结构不变
      folders.value = defaultFolders.map(df => {
        const match = apiFolders.find((f: MailFolder) =>
          f.id.toLowerCase() === df.id.toLowerCase() ||
          f.displayName === df.displayName ||
          folderNameMap[f.displayName] === df.displayName
        )
        return match ? { ...df, totalItemCount: match.totalItemCount, unreadItemCount: match.unreadItemCount } : df
      })
    } catch (e: any) {
      console.error('Load folders error:', e)
      error.value = String(e)
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
   */
  async function loadMessages(accountId: number, folderId: string, page = 0) {
    loading.value = true
    try {
      // @ts-ignore
      const msgs = await window.go.main.App.GetMessages(accountId, folderId, page) || []
      // 首页替换，后续页追加
      if (page === 0) {
        messages.value = msgs
      } else {
        messages.value = [...messages.value, ...msgs]
      }
      currentPage.value = page
    } catch (e: any) {
      console.error('Load messages error:', e)
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
   */
  async function loadMessageDetail(accountId: number, messageId: string) {
    const cacheKey = `${accountId}-${messageId}`
    const cached = messageCache.get(cacheKey)
    if (cached) {
      currentMessage.value = cached.message
      attachments.value = cached.attachments
      return
    }

    try {
      // 并行加载邮件详情和附件
      // @ts-ignore
      const [msg, atts] = await Promise.all([
        window.go.main.App.GetMessageDetail(accountId, messageId),
        window.go.main.App.GetAttachments(accountId, messageId)
      ])
      currentMessage.value = msg
      attachments.value = atts || []
      messageCache.set(cacheKey, { message: msg, attachments: atts || [] })
    } catch (e: any) {
      console.error('Load message detail error:', e)
    }
  }

  /**
   * 重置所有邮件状态
   */
  function reset() {
    folders.value = [...defaultFolders]
    messages.value = []
    currentMessage.value = null
    attachments.value = []
    selectedFolderId.value = null
    currentPage.value = 0
    error.value = null
  }

  return {
    folders, messages, currentMessage, attachments, selectedFolderId,
    loading, currentPage, error,
    loadFolders, loadMessages, loadMessageDetail, reset
  }
})
