/**
 * AI/LLM related types for streaming chat conversations.
 *
 * This namespace provides types for interacting with AI models through Wox's
 * streaming chat interface. It supports conversation management, streaming
 * responses, and tool calling capabilities.
 *
 * @example
 * ```typescript
 * import { AI, PublicAPI } from "wox-plugin"
 *
 * // Prepare conversation history
 * const conversations: AI.Conversation[] = [
 *   { Role: "system", Text: "You are a helpful assistant.", Timestamp: Date.now() },
 *   { Role: "user", Text: "What is the capital of France?", Timestamp: Date.now() }
 * ]
 *
 * // Stream the response
 * await api.LLMStream(ctx, conversations, (streamData) => {
 *   if (streamData.Status === "streaming") {
 *     console.log("Chunk:", streamData.Data)
 *   } else if (streamData.Status === "finished") {
 *     console.log("Complete:", streamData.Data)
 *   }
 * })
 * ```
 */
export namespace AI {
  /**
   * Role of a message in a conversation.
   *
   * - `system`: System prompt that sets AI behavior
   * - `user`: Message from the user
   * - `assistant`: Response from the AI model
   * - `tool`: Result from a tool/function call execution
   */
  export type ConversationRole = "user" | "system" | "assistant" | "tool"

  /**
   * Status of a streaming chat response.
   *
   * - `streaming`: actively receiving response chunks
   * - `streamed`: all content received, ready to process tool calls (if any)
   * - `running_tool_call`: executing tool calls
   * - `finished`: all content and tool calls completed
   * - `error`: an error occurred during streaming or tool execution
   */
  export type ChatStreamDataType = "streaming" | "streamed" | "running_tool_call" | "finished" | "error"

  /**
   * Status of a tool call execution.
   *
   * - `streaming`: tool call arguments are being streamed
   * - `pending`: streaming finished, ready to execute
   * - `running`: currently executing
   * - `succeeded`: executed successfully
   * - `failed`: execution failed
   */
  export type ToolCallStatus = "streaming" | "pending" | "running" | "succeeded" | "failed"

  /**
   * Represents a single message in an AI conversation.
   *
   * Conversations are passed to the AI model in order to maintain context
   * across multiple turns of dialogue.
   *
   * @example
   * ```typescript
   * const systemMessage: AI.Conversation = {
   *   Role: "system",
   *   Text: "You are a helpful coding assistant.",
   *   Timestamp: Date.now()
   * }
   *
   * const userMessage: AI.Conversation = {
   *   Role: "user",
   *   Text: "How do I reverse a string in JavaScript?",
   *   Timestamp: Date.now()
   * }
   * ```
   */
  export interface Conversation {
    /**
     * The role of the message sender.
     *
     * - `system`: Sets behavior/context for the AI
     * - `user`: Human input
     * - `assistant`: AI response
     * - `tool`: Result from tool execution
     */
    Role: ConversationRole

    /**
     * The text content of the message.
     *
     * For system messages, this defines the AI's behavior.
     * For user messages, this is the user's input.
     * For assistant messages, this is the AI's response.
     * For tool messages, this is the stringified tool result.
     */
    Text: string

    /**
     * Reasoning content from models that support reasoning (e.g., DeepSeek, OpenAI o1, Qwen).
     *
     * This is the model's internal thinking process, separate from the final response.
     * It's useful for understanding how the model arrived at its answer.
     *
     * Only present for assistant messages when using models that support reasoning.
     */
    Reasoning?: string

    /**
     * List of PNG image bytes for vision models.
     *
     * Only applicable for user messages when using vision-capable models.
     * Images should be in PNG format as raw byte arrays.
     */
    Images?: Uint8Array[]

    /**
     * Tool call information for assistant or tool messages.
     *
     * - For assistant messages: describes which tool(s) the AI wants to call
     * - For tool messages: describes the result of a tool execution
     */
    ToolCallInfo?: ToolCallInfo

    /**
     * Unix timestamp in milliseconds when this message was created.
     *
     * Used for ordering messages in the conversation history.
     */
    Timestamp: number
  }

  /**
   * Information about a tool/function call requested by the AI model.
   *
   * When the AI decides to use a tool, it populates this structure with
   * the tool name and arguments. After execution, the result is stored here.
   *
   * @example
   * ```typescript
   * const toolCall: AI.ToolCallInfo = {
   *   Id: "call_123",
   *   Name: "get_weather",
   *   Arguments: { city: "London", unit: "celsius" },
   *   Status: "running",
   *   Delta: "",
   *   Response: "",
   *   StartTimestamp: Date.now(),
   *   EndTimestamp: 0
   * }
   * ```
   */
  export interface ToolCallInfo {
    /**
     * Unique identifier for this tool call.
     *
     * Used to match tool requests with responses.
     */
    Id: string

    /**
     * Name of the tool/function to call.
     *
     * This should match a registered tool name in the Wox system.
     */
    Name: string

    /**
     * Arguments to pass to the tool function.
     *
     * Key-value pairs representing the parameters for the tool call.
     * The schema depends on the tool's definition.
     */
    Arguments: Record<string, any>

    /**
     * Current status of the tool call.
     *
     * Progresses through: streaming -> pending -> running -> succeeded/failed
     */
    Status: ToolCallStatus

    /**
     * Delta content when tool call is being streamed.
     *
     * As the model streams the tool call arguments, partial content
     * appears here. Once complete, Arguments contains the full data.
     */
    Delta: string

    /**
     * The response/result from tool execution.
     *
     * After the tool completes, this contains the stringified result
     * that will be sent back to the AI model.
     */
    Response: string

    /**
     * Unix timestamp in milliseconds when tool execution started.
     */
    StartTimestamp: number

    /**
     * Unix timestamp in milliseconds when tool execution finished.
     *
     * Zero if the tool hasn't finished yet.
     */
    EndTimestamp: number
  }

  /**
   * Data returned during a streaming chat response.
   *
   * As the AI model generates its response, this structure is updated
   * with new content. It supports regular text, reasoning content,
   * and tool calling.
   *
   * @example
   * ```typescript
   * await api.LLMStream(ctx, conversations, (data: AI.ChatStreamData) => {
   *   switch (data.Status) {
   *     case "streaming":
   *       // Still receiving content
   *       process.stdout.write(data.Data)
   *       break
   *     case "streamed":
   *       // All content received, checking for tool calls
   *       console.log("\nContent complete. Tool calls:", data.ToolCalls)
   *       break
   *     case "running_tool_call":
   *       // Executing tools
   *       data.ToolCalls.forEach(tc => {
   *         console.log(`Running ${tc.Name}: ${tc.Status}`)
   *       })
   *       break
   *     case "finished":
   *       // Completely done
   *       console.log("\nFinal result:", data.Data)
   *       break
   *     case "error":
   *       console.error("Error:", data.Data)
   *       break
   *   }
   * })
   * ```
   */
  export interface ChatStreamData {
    /**
     * Current status of the stream.
     *
     * Indicates what phase the chat is in: streaming content,
     * executing tools, completed, or errored.
     */
    Status: ChatStreamDataType

    /**
     * Aggregated text content from the AI.
     *
     * This field accumulates content as it's streamed. For example:
     * - Chunk 1: Data = "Hello"
     * - Chunk 2: Data = "Hello world"
     * - Chunk 3: Data = "Hello world!"
     *
     * Use this for the final complete response.
     */
    Data: string

    /**
     * Reasoning content from models that support thinking.
     *
     * Models like DeepSeek-R1, OpenAI o1, and Qwen separate their
     * internal reasoning from the final response. This field contains
     * the model's thinking process.
     *
     * To display reasoning nicely, you can format it with a "> " prefix:
     * ```typescript
     * const reasoning = data.Reasoning
     *   .split('\n')
     *   .map(line => `> ${line}`)
     *   .join('\n')
     * ```
     */
    Reasoning: string

    /**
     * Tool calls requested by the AI model.
     *
     * Populated when the model decides to use one or more tools.
     * Each tool call includes its name, arguments, and execution status.
     *
     * @example
     * ```typescript
     * if (data.ToolCalls.length > 0) {
     *   for (const toolCall of data.ToolCalls) {
     *     console.log(`Tool: ${toolCall.Name}`)
     *     console.log(`Args: ${JSON.stringify(toolCall.Arguments)}`)
     *     console.log(`Status: ${toolCall.Status}`)
     *   }
     * }
     * ```
     */
    ToolCalls: ToolCallInfo[]
  }

  /**
   * Callback function type for receiving streaming chat data.
   *
   * Passed to `PublicAPI.LLMStream()` and called repeatedly as
   * the AI generates its response.
   *
   * @param streamData - The current streaming data chunk
   *
   * @example
   * ```typescript
   * const callback: AI.ChatStreamFunc = (streamData) => {
   *   if (streamData.Status === "streaming") {
   *     // Append to display
   *     display.textContent = streamData.Data
   *   }
   * }
   *
   * await api.LLMStream(ctx, conversations, callback)
   * ```
   */
  export type ChatStreamFunc = (streamData: ChatStreamData) => void

  /**
   * Definition of an AI model (provider and model name).
   *
   * Used when specifying which model to use for chat completions.
   *
   * @example
   * ```typescript
   * const model: AI.AIModel = {
   *   Provider: "openai",
   *   ProviderAlias: "my-openai-account", // optional, for multiple configs
   *   Name: "gpt-4"
   * }
   * ```
   */
  export interface AIModel {
    /**
     * The model provider name.
     *
     * Common values: "openai", "anthropic", "deepseek", "ollama", etc.
     */
    Provider: string

    /**
     * Optional provider alias.
     *
     * When you have multiple configurations of the same provider
     * (e.g., two different OpenAI API keys), use this to specify
     * which one to use. If omitted, uses the default configuration.
     */
    ProviderAlias?: string

    /**
     * The specific model name.
     *
     * Examples: "gpt-4", "claude-3-opus", "deepseek-chat", "llama3:70b"
     */
    Name: string
  }
}
