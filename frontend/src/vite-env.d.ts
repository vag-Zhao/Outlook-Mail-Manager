/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

interface Window {
  go: {
    main: {
      App: {
        ImportAccounts(content: string): Promise<number>
        GetAccounts(groupId: number | null): Promise<any[]>
        DeleteAccount(id: number): Promise<void>
        DeleteAccounts(ids: number[]): Promise<void>
        GetAccountCount(): Promise<number>
        MoveAccountToGroup(accountId: number, groupId: number): Promise<void>
        MoveAccountsToGroup(ids: number[], groupId: number): Promise<void>
        CheckAccountToken(accountId: number): Promise<boolean>
        GetGroups(): Promise<any[]>
        CreateGroup(name: string): Promise<any>
        UpdateGroup(id: number, name: string): Promise<void>
        DeleteGroup(id: number): Promise<void>
        ClearGroup(groupId: number): Promise<void>
        GetMailFolders(accountId: number): Promise<any[]>
        GetMessages(accountId: number, folderId: string, page: number): Promise<any[]>
        SearchMessages(accountId: number, folderId: string, keyword: string): Promise<any[]>
        GetMessageDetail(accountId: number, messageId: string): Promise<any>
        GetAttachments(accountId: number, messageId: string): Promise<any[]>
        DeleteMessage(accountId: number, messageId: string): Promise<void>
        MarkMessageRead(accountId: number, messageId: string, isRead: boolean): Promise<void>
        SaveFile(content: string): Promise<boolean>
      }
    }
  }
}
