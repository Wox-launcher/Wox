export namespace AI {
  export type ConversationRole = "user" | "system"
  export type ChatStreamDataType = "streaming" | "finished" | "error"

  export interface Conversation {
    Role: ConversationRole
    Text: string
    Timestamp: number
  }

  export type ChatStreamFunc = (dataType: ChatStreamDataType, data: string) => void
}
