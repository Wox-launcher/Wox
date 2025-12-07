export namespace AI {
  export type ConversationRole = "user" | "system"
  export type ChatStreamDataType = "streaming" | "finished" | "error"

  export interface Conversation {
    Role: ConversationRole
    Text: string
    Timestamp: number
  }

  export interface ChatStreamData {
    /** Stream status */
    Status: ChatStreamDataType
    /** Aggregated content data */
    Data: string
    /** Reasoning content from models that support reasoning (e.g., DeepSeek, OpenAI o1). Separate from Data for clean processing. */
    Reasoning: string
  }

  export type ChatStreamFunc = (streamData: ChatStreamData) => void
}
