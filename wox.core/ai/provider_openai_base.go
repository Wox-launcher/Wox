package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"
	"wox/common"
	"wox/setting"
	"wox/util"

	"github.com/tmc/langchaingo/jsonschema"
)

type OpenAIBaseProviderOptions struct {
	Headers              map[string]string
	ModelsQueryParams    map[string]string
	ChatRequestExtraBody func(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions) map[string]any
}

// OpenAIBaseProvider is the base provider for all OpenAI compatible providers
type OpenAIBaseProvider struct {
	connectContext setting.AIProvider
	options        OpenAIBaseProviderOptions
}

// OpenAIBaseProviderStream represents a stream from OpenAI compatible providers
type OpenAIBaseProviderStream struct {
	stream               *sseReader
	conversations        []common.Conversation
	toolCalls            []openAIAccumulatedToolCall
	sawExplicitToolIndex bool
	accumulatedReason    string // accumulated reasoning content from chunks
	accumulatedData      string // accumulated answer content after provider-specific reasoning markers are removed
	pendingData          string // buffered content that may contain a split content tag marker across stream chunks
	activeContentTag     streamContentTag
	hasContentTag        bool // true when content chunks are currently inside a configured tagged block
	trimAnswerPrefix     bool // true after a tagged block so separator whitespace is not kept as answer text
}

type streamContentTarget string

const (
	streamContentTargetReasoning streamContentTarget = "reasoning"
)

type streamContentTag struct {
	Start  string
	End    string
	Target streamContentTarget
}

var streamContentTags = []streamContentTag{
	// Some providers expose reasoning through content tags instead of the
	// structured delta.reasoning field. Keep tags declarative so supporting a new
	// provider-specific marker only requires adding one entry here.
	{Start: "<think>", End: "</think>", Target: streamContentTargetReasoning},
}

var reasoningExtraFieldNames = [...]string{
	// OpenAI-compatible providers do not agree on one streamed reasoning field.
	// Keep the known names in one small list so receiving chunks and empty-chunk
	// detection stay in sync when provider-specific support is added.
	"reasoning",
	"reasoning_content",
}

// NewOpenAIBaseProvider creates a new OpenAI base provider
func NewOpenAIBaseProvider(connectContext setting.AIProvider) *OpenAIBaseProvider {
	return &OpenAIBaseProvider{connectContext: connectContext}
}

func NewOpenAIBaseProviderWithOptions(connectContext setting.AIProvider, options OpenAIBaseProviderOptions) *OpenAIBaseProvider {
	return &OpenAIBaseProvider{connectContext: connectContext, options: options}
}

// ChatStream starts a chat stream with the OpenAI compatible provider
func (o *OpenAIBaseProvider) ChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions) (ChatStream, error) {
	util.GetLogger().Debug(ctx, fmt.Sprintf("AI: chat stream with model: %s, conversations: %d, tools: %d", model.Name, len(conversations), len(options.Tools)))

	for i, conv := range conversations {
		util.GetLogger().Debug(ctx, fmt.Sprintf("AI: conversation[%d] - role: %s, text: %s, toolCallID: %s", i, conv.Role, conv.Text, conv.ToolCallInfo.Id))
	}
	convertedTools := o.convertTools(options.Tools)
	for i, tool := range convertedTools {
		util.GetLogger().Debug(ctx, fmt.Sprintf("AI: converted tool[%d] name: %s, paramters: %v", i, tool.Function.Name, tool.Function.Parameters))
	}

	messages, err := o.convertConversations(conversations)
	if err != nil {
		return nil, err
	}

	reqBody, err := o.buildChatRequestBody(ctx, model, conversations, messages, convertedTools, options, true)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := o.doRequest(ctx, http.MethodPost, "chat/completions", nil, bodyBytes, "text/event-stream")
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("AI: Stream request error: %s", err.Error()))
		return nil, err
	}

	return &OpenAIBaseProviderStream{
		conversations: conversations,
		stream:        newSSEReader(resp.Body),
	}, nil
}

func (o *OpenAIBaseProvider) buildChatRequestBody(ctx context.Context, model common.Model, conversations []common.Conversation, messages []openAIChatMessage, tools []openAITool, options common.ChatOptions, stream bool) (map[string]any, error) {
	body := map[string]any{
		"model":    model.Name,
		"messages": messages,
		"stream":   stream,
	}
	if len(tools) > 0 {
		body["tools"] = tools
		body["tool_choice"] = "auto"
	}
	for k, v := range o.getChatRequestExtraBody(ctx, model, conversations, options) {
		body[k] = v
	}
	return body, nil
}

func (o *OpenAIBaseProvider) getChatRequestExtraBody(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions) map[string]any {
	if o.options.ChatRequestExtraBody == nil {
		return nil
	}

	return o.options.ChatRequestExtraBody(ctx, model, conversations, options)
}

// Models returns the list of available models from the OpenAI compatible provider
func (o *OpenAIBaseProvider) Models(ctx context.Context) ([]common.Model, error) {
	var listResp openAIModelsListResponse
	if err := o.doJSONRequest(ctx, http.MethodGet, "models", o.options.ModelsQueryParams, nil, &listResp); err != nil {
		return nil, err
	}

	openaiModels := make([]common.Model, 0, len(listResp.Data))
	for _, model := range listResp.Data {
		openaiModels = append(openaiModels, common.Model{
			Name:          model.ID,
			Provider:      common.ProviderName(o.connectContext.Name),
			ProviderAlias: o.connectContext.Alias,
		})
	}

	return openaiModels, nil
}

// Ping checks if the OpenAI compatible provider is available
func (o *OpenAIBaseProvider) Ping(ctx context.Context) error {
	return o.doJSONRequest(ctx, http.MethodGet, "models", nil, nil, nil)
}

func (o *OpenAIBaseProvider) pingChatCompletion(ctx context.Context, modelName string) error {
	reqBody := map[string]any{
		"model": modelName,
		"messages": []openAIChatMessage{
			{
				Role:    "user",
				Content: stringPtr("ping"),
			},
		},
		"max_completion_tokens": 1,
	}
	return o.doJSONRequest(ctx, http.MethodPost, "chat/completions", nil, reqBody, nil)
}

func (o *OpenAIBaseProvider) convertTools(tools []common.Tool) []openAITool {
	convertedTools := make([]openAITool, len(tools))
	for i, tool := range tools {
		convertedTools[i] = openAITool{
			Type: "function",
			Function: openAIToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  toolInputSchema(tool),
			},
		}
	}
	return convertedTools
}

// toolInputSchema preserves full plugin contracts while keeping legacy tool schemas compatible.
func toolInputSchema(tool common.Tool) map[string]any {
	if tool.InputSchema != nil {
		return tool.InputSchema
	}
	return jsonSchemaDefinitionToMap(tool.Parameters)
}

// jsonSchemaDefinitionToMap copies a tool schema without langchaingo's empty
// `properties` on non-object types. Strict OpenAI-compatible hosts such as
// SiliconFlow reject that extra field with HTTP 400.
func jsonSchemaDefinitionToMap(def jsonschema.Definition) map[string]any {
	result := map[string]any{}
	if def.Type != "" {
		result["type"] = string(def.Type)
	}
	if def.Description != "" {
		result["description"] = def.Description
	}
	if len(def.Enum) > 0 {
		result["enum"] = def.Enum
	}
	if def.Type == jsonschema.Object || len(def.Properties) > 0 {
		properties := map[string]any{}
		for name, property := range def.Properties {
			properties[name] = jsonSchemaDefinitionToMap(property)
		}
		result["properties"] = properties
		if _, ok := result["type"]; !ok {
			result["type"] = string(jsonschema.Object)
		}
	}
	if len(def.Required) > 0 {
		result["required"] = def.Required
	}
	if def.Items != nil {
		result["items"] = jsonSchemaDefinitionToMap(*def.Items)
	}
	return result
}

func (s *OpenAIBaseProviderStream) Receive(ctx context.Context) (common.ChatStreamData, error) {
	rawEvent, err := s.stream.nextEvent()
	if err != nil {
		if !errors.Is(err, io.EOF) {
			util.GetLogger().Error(ctx, fmt.Sprintf("AI: Stream error: %s", err.Error()))
			return common.ChatStreamData{}, err
		}

		var toolCallInfos []common.ToolCallInfo
		if len(s.toolCalls) > 0 {
			util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Tool call streaming finished, tool calls count: %d", len(s.toolCalls)))

			for _, toolCall := range s.toolCalls {
				toolCallInfo := common.ToolCallInfo{
					Id:    toolCall.ID,
					Name:  toolCall.Name,
					Delta: toolCall.Arguments,
				}

				// try to unmarshal tool call arguments if possible
				var argsMap map[string]any
				if err := json.Unmarshal([]byte(toolCall.Arguments), &argsMap); err == nil {
					toolCallInfo.Arguments = s.normalizeArguments(ctx, toolCall.Name, argsMap)
					toolCallInfo.Status = common.ToolCallStatusPending
				} else {
					util.GetLogger().Error(ctx, fmt.Sprintf("AI: Failed to unmarshal tool call arguments, json=%s, err: %s", toolCall.Arguments, err.Error()))
					toolCallInfo.Arguments = map[string]any{}
					toolCallInfo.Status = common.ToolCallStatusFailed
					toolCallInfo.Response = err.Error()
				}

				toolCallInfos = append(toolCallInfos, toolCallInfo)
			}
		}

		// Some OpenAI-compatible providers stream reasoning as tagged content
		// instead of using delta.reasoning. Flush any incomplete marker text at
		// stream end so normal answers are not lost, while completed tagged blocks
		// stay separated from the user-visible answer.
		s.flushPendingData()
		finalContent := s.accumulatedData

		util.GetLogger().Debug(ctx, "AI: Stream ended, final message received"+finalContent)
		return common.ChatStreamData{
			Status:    common.ChatStreamStatusStreamed,
			Data:      finalContent,
			Reasoning: s.accumulatedReason,
			ToolCalls: toolCallInfos,
		}, nil
	}

	util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Received raw chunk: %s", string(rawEvent)))

	var chunk openAIChatCompletionChunk
	if unmarshalErr := json.Unmarshal(rawEvent, &chunk); unmarshalErr != nil {
		s.stream.close()
		util.GetLogger().Error(ctx, fmt.Sprintf("AI: Failed to unmarshal stream chunk: %s, err: %s", string(rawEvent), unmarshalErr.Error()))
		return common.ChatStreamData{}, unmarshalErr
	}

	if chunk.Error != nil && len(chunk.Choices) == 0 {
		s.stream.close()
		msg := strings.TrimSpace(chunk.Error.Message)
		if msg == "" {
			msg = string(rawEvent)
		}
		util.GetLogger().Error(ctx, fmt.Sprintf("AI: Stream error chunk: %s", msg))
		return common.ChatStreamData{}, errors.New(msg)
	}

	// Store previous normalized content and reasoning before adding chunk so
	// tag-only chunks do not look like user-visible answer updates.
	previousContent := s.accumulatedData
	previousReasoning := s.accumulatedReason

	// Extract reasoning, content, and tool calls from current chunk if present
	if len(chunk.Choices) > 0 {
		delta := chunk.Choices[0].Delta

		for _, reasoningFieldName := range reasoningExtraFieldNames {
			if rawReasoning, exists := delta.reasoningRawField(reasoningFieldName); exists {
				// DeepSeek streams thinking as reasoning_content while some other
				// OpenAI-compatible providers use reasoning. Keep compatible field
				// names together and parse them through the same path.
				var reasoningStr string
				if err := json.Unmarshal(rawReasoning, &reasoningStr); err == nil {
					if reasoningStr != "" {
						s.accumulatedReason += reasoningStr
						util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Extracted reasoning from chunk field %s: %s", reasoningFieldName, reasoningStr))
					}
				} else {
					util.GetLogger().Error(ctx, fmt.Sprintf("AI: Failed to unmarshal reasoning field %s: %s, error: %s", reasoningFieldName, string(rawReasoning), err.Error()))
				}
			}
		}

		if delta.Content != "" {
			s.appendContentDelta(delta.Content)
		}

		if len(delta.ToolCalls) > 0 {
			s.accumulateToolCalls(delta.ToolCalls)
		}
	}

	// Check if normalized answer content has changed after processing the chunk.
	currentContent := s.accumulatedData

	// If neither content nor reasoning has changed and there are no tool calls, skip this chunk
	if currentContent == previousContent && s.accumulatedReason == previousReasoning && s.isChunkEmpty(chunk) {
		return common.ChatStreamData{}, ChatStreamNoContentErr
	}

	// Keep reasoning and content separate in ChatStreamData
	streamData := common.ChatStreamData{
		Status:    common.ChatStreamStatusStreaming,
		Data:      currentContent,
		Reasoning: s.accumulatedReason,
	}
	totalToolCallCount := len(s.toolCalls)
	if totalToolCallCount > 0 {
		toolCallInfos := make([]common.ToolCallInfo, 0, totalToolCallCount)
		for index, toolcall := range s.toolCalls {
			isLastToolCall := index == totalToolCallCount-1

			// if the toolcall is not the last one, we will set the status to pending, because the tool call streaming is one by one
			// the prev toolcall streaming must be finished before the next one
			status := common.ToolCallStatusStreaming
			if totalToolCallCount > 1 && !isLastToolCall {
				status = common.ToolCallStatusPending
			}

			toolCallInfo := common.ToolCallInfo{
				Id:        toolcall.ID,
				Name:      toolcall.Name,
				Arguments: map[string]any{},
				Delta:     toolcall.Arguments,
				Status:    status,
			}
			toolCallInfos = append(toolCallInfos, toolCallInfo)
		}
		streamData.ToolCalls = toolCallInfos
	}

	return streamData, nil
}

func (s *OpenAIBaseProviderStream) accumulateToolCalls(toolCalls []openAIChunkToolCall) {
	for _, tc := range toolCalls {
		if tc.Index > 0 {
			s.sawExplicitToolIndex = true
		}

		idx := tc.Index
		if idx < 0 {
			idx = 0
		}
		if !s.sawExplicitToolIndex && tc.Index == 0 && len(s.toolCalls) > 0 {
			if tc.ID != "" {
				matched := -1
				for i, existing := range s.toolCalls {
					if existing.ID == tc.ID {
						matched = i
						break
					}
				}
				if matched >= 0 {
					idx = matched
				} else if s.toolCalls[0].ID != "" {
					idx = len(s.toolCalls)
				}
			} else if len(s.toolCalls) > 1 {
				// Index-less relays omit index and ID on continuation argument chunks.
				idx = len(s.toolCalls) - 1
			}
		}

		for len(s.toolCalls) <= idx {
			s.toolCalls = append(s.toolCalls, openAIAccumulatedToolCall{})
		}
		if tc.ID != "" && s.toolCalls[idx].ID == "" {
			s.toolCalls[idx].ID = tc.ID
		}
		if tc.Function.Name != "" {
			s.toolCalls[idx].Name += tc.Function.Name
		}
		if tc.Function.Arguments != "" {
			s.toolCalls[idx].Arguments += tc.Function.Arguments
		}
	}
}

func (s *OpenAIBaseProviderStream) appendContentDelta(delta string) {
	s.pendingData += delta

	for s.pendingData != "" {
		if s.hasContentTag {
			if markerIndex := strings.Index(s.pendingData, s.activeContentTag.End); markerIndex >= 0 {
				s.appendTaggedContent(s.pendingData[:markerIndex])
				s.pendingData = s.pendingData[markerIndex+len(s.activeContentTag.End):]
				s.hasContentTag = false
				if s.accumulatedData == "" {
					s.trimAnswerPrefix = true
				}
				continue
			}

			keepLength := s.contentMarkerPrefixLength(s.pendingData, []string{s.activeContentTag.End})
			flushLength := len(s.pendingData) - keepLength
			if flushLength > 0 {
				s.appendTaggedContent(s.pendingData[:flushLength])
				s.pendingData = s.pendingData[flushLength:]
			}
			break
		}

		tag, markerIndex, found := s.findFirstContentTagStart(s.pendingData)
		if found {
			s.appendNormalizedContent(s.pendingData[:markerIndex])
			s.pendingData = s.pendingData[markerIndex+len(tag.Start):]
			s.activeContentTag = tag
			s.hasContentTag = true
			continue
		}

		keepLength := s.contentMarkerPrefixLength(s.pendingData, s.contentTagStartMarkers())
		flushLength := len(s.pendingData) - keepLength
		if flushLength > 0 {
			s.appendNormalizedContent(s.pendingData[:flushLength])
			s.pendingData = s.pendingData[flushLength:]
		}
		break
	}
}

func (s *OpenAIBaseProviderStream) appendNormalizedContent(content string) {
	if content == "" {
		return
	}

	if s.trimAnswerPrefix {
		// Models commonly emit whitespace after a reasoning tag. Drop only that
		// separator before the first answer token so Data starts with the actual
		// response.
		content = strings.TrimLeftFunc(content, unicode.IsSpace)
		if content == "" {
			return
		}
		s.trimAnswerPrefix = false
	}

	s.accumulatedData += content
}

func (s *OpenAIBaseProviderStream) appendTaggedContent(content string) {
	if content == "" {
		return
	}

	s.appendRoutedContent(s.activeContentTag.Target, content)
}

func (s *OpenAIBaseProviderStream) appendRoutedContent(target streamContentTarget, content string) {
	if content == "" {
		return
	}

	// Routed content is controlled by configuration. Today all supported routes
	// carry reasoning, but this switch keeps the parser extensible if a future
	// provider adds another content category.
	switch target {
	case streamContentTargetReasoning:
		s.accumulatedReason += content
	default:
		s.accumulatedData += content
	}
}

func (s *OpenAIBaseProviderStream) flushPendingData() {
	if s.pendingData == "" {
		return
	}

	if s.hasContentTag {
		s.appendTaggedContent(s.pendingData)
	} else {
		s.appendNormalizedContent(s.pendingData)
	}
	s.pendingData = ""
}

func (s *OpenAIBaseProviderStream) findFirstContentTagStart(content string) (streamContentTag, int, bool) {
	var matchedTag streamContentTag
	matchedIndex := -1

	for _, tag := range streamContentTags {
		if tag.Start == "" || tag.End == "" {
			continue
		}

		if index := strings.Index(content, tag.Start); index >= 0 && (matchedIndex == -1 || index < matchedIndex) {
			matchedTag = tag
			matchedIndex = index
		}
	}

	return matchedTag, matchedIndex, matchedIndex >= 0
}

func (s *OpenAIBaseProviderStream) contentTagStartMarkers() []string {
	markers := make([]string, 0, len(streamContentTags))
	for _, tag := range streamContentTags {
		if tag.Start != "" && tag.End != "" {
			markers = append(markers, tag.Start)
		}
	}

	return markers
}

func (s *OpenAIBaseProviderStream) contentMarkerPrefixLength(content string, markers []string) int {
	maxPrefixLength := 0

	for _, marker := range markers {
		maxLength := len(marker) - 1
		if len(content) < maxLength {
			maxLength = len(content)
		}

		for length := maxLength; length > 0; length-- {
			if strings.HasSuffix(content, marker[:length]) && length > maxPrefixLength {
				maxPrefixLength = length
			}
		}
	}

	return maxPrefixLength
}

// normalizeArguments normalizes the tool call arguments
// Case 1:
//
//		because we unmarshal the tool call arguments as map[string]any, some types are not correct, E.g. int64 will be unmarshaled as float64
//	 so we need to normalize the types base on the tool call definition
//
// Case 2:
//
//	the model does not always generate valid JSON, and may hallucinate parameters not defined by your function schema.
//
// E.g. {"sequenceNumber": 123} -> {"sequence_number": 123}
//
// Case 3:
//
//	sometimes required arguments are not provided, so we need to add them to the arguments
func (s *OpenAIBaseProviderStream) normalizeArguments(ctx context.Context, toolName string, argsMap map[string]any) map[string]any {
	util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Start normalizing tool call arguments for tool: %s, args: %v", toolName, argsMap))

	var tool common.Tool
	if t, ok := GetToolRegistry().Get(toolName); ok {
		tool = t
	}

	if tool.Name == "" {
		util.GetLogger().Error(ctx, fmt.Sprintf("AI: Tool not found: %s", toolName))
		return argsMap
	}

	// fix argument types
	for toolRequiredName, param := range tool.Parameters.Properties {
		if param.Type == "integer" {
			// name sometimes is not the same as the tool call argument name, so we need to map the name to the tool call argument name
			// E.g. sequenceNumber -> sequence_number
			for aiReturnName, value := range argsMap {
				if s.isToolCallArgumentNameSame(toolRequiredName, aiReturnName) {
					if f, ok := value.(float64); ok {
						argsMap[toolRequiredName] = int64(f)
						util.GetLogger().Debug(ctx, fmt.Sprintf("AI: argument type fixed %s, from float to int", toolRequiredName))
					}
				}
			}
		}
	}

	// fix required arguments
	for _, requiredName := range tool.Parameters.Required {
		if _, ok := argsMap[requiredName]; !ok {
			// add the required argument to the arguments based on the property definition
			if prop, ok := tool.Parameters.Properties[requiredName]; ok {
				if prop.Type == "string" {
					argsMap[requiredName] = ""
				} else if prop.Type == "integer" {
					argsMap[requiredName] = int64(0)
				} else if prop.Type == "object" {
					argsMap[requiredName] = map[string]any{}
				} else if prop.Type == "array" {
					argsMap[requiredName] = []any{}
				} else if prop.Type == "boolean" {
					argsMap[requiredName] = false
				} else {
					argsMap[requiredName] = nil
				}

				util.GetLogger().Debug(ctx, fmt.Sprintf("AI: required argument %s missing, added with default value: %s", requiredName, argsMap[requiredName]))
			} else {
				argsMap[requiredName] = nil
			}
		}
	}

	util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Normalized tool call arguments successfully, args: %v", argsMap))

	return argsMap
}

func (s *OpenAIBaseProviderStream) isToolCallArgumentNameSame(toolRequiredName string, aiReturnName string) bool {
	if strings.EqualFold(toolRequiredName, aiReturnName) {
		return true
	}

	// name sometimes is not the same as the tool call argument name, so we need to map the name to the tool call argument name
	// E.g. sequenceNumber -> sequence_number
	if strings.EqualFold(strings.ReplaceAll(toolRequiredName, "_", ""), strings.ReplaceAll(aiReturnName, "_", "")) {
		return true
	}

	return false
}

func (s *OpenAIBaseProviderStream) isChunkEmpty(chunk openAIChatCompletionChunk) bool {
	if len(chunk.Choices) == 0 {
		return true
	}

	delta := chunk.Choices[0].Delta

	// Check regular fields
	if delta.Content != "" || delta.Refusal != "" || len(delta.ToolCalls) > 0 {
		return false
	}

	// Check for reasoning fields. DeepSeek uses reasoning_content, while other
	// OpenAI-compatible providers use reasoning, and both should keep streaming
	// chunks alive even when there is no user-visible content.
	for _, reasoningFieldName := range reasoningExtraFieldNames {
		if _, exists := delta.reasoningRawField(reasoningFieldName); exists {
			return false
		}
	}

	return true
}

// convertConversations converts the conversations to OpenAI format
func (o *OpenAIBaseProvider) convertConversations(conversations []common.Conversation) ([]openAIChatMessage, error) {
	var chatMessages []openAIChatMessage
	pendingToolContent := ""
	pendingToolReasoning := ""
	for i := 0; i < len(conversations); i++ {
		conversation := conversations[i]
		if conversation.Role == common.ConversationRoleSystem {
			text := conversation.Text
			if i == 0 {
				// Some model templates only accept a single system message at the beginning.
				for i+1 < len(conversations) && conversations[i+1].Role == common.ConversationRoleSystem {
					i++
					text += "\n\n" + conversations[i].Text
				}
			}
			chatMessages = append(chatMessages, openAIChatMessage{
				Role:    "system",
				Content: stringPtr(text),
			})
		}
		if conversation.Role == common.ConversationRoleUser {
			images := append([]common.WoxImage(nil), conversation.Images...)
			for _, attachment := range conversation.Attachments {
				if attachment.Kind != common.AIChatAttachmentImage {
					continue
				}
				path := common.ChatAttachmentPath(attachment)
				if path == "" {
					return nil, fmt.Errorf("invalid image attachment: %s", attachment.Name)
				}
				images = append(images, common.NewWoxImageAbsolutePath(path))
			}
			if len(images) == 0 {
				chatMessages = append(chatMessages, openAIChatMessage{
					Role:    "user",
					Content: stringPtr(conversation.Text),
				})
			} else {
				parts := []openAIContentPart{{Type: "text", Text: conversation.Text}}
				for _, image := range images {
					imageURL, err := chatImageURL(image)
					if err != nil {
						return nil, fmt.Errorf("read chat image: %w", err)
					}
					parts = append(parts, openAIContentPart{
						Type:     "image_url",
						ImageURL: &openAIContentImage{URL: imageURL},
					})
				}
				chatMessages = append(chatMessages, openAIChatMessage{
					Role:    "user",
					Content: parts,
				})
			}
		}
		if conversation.Role == common.ConversationRoleAssistant {
			if o.shouldFoldAssistantIntoNextToolCall(conversations, i) {
				pendingToolContent = conversation.Text
				pendingToolReasoning = conversation.Reasoning
				continue
			}
			chatMessages = append(chatMessages, o.convertAssistantMessage(conversation.Text, conversation.Reasoning))
		}
		if conversation.Role == common.ConversationRoleTool {
			toolConversations := []common.Conversation{conversation}
			for i+1 < len(conversations) && conversations[i+1].Role == common.ConversationRoleTool {
				i++
				toolConversations = append(toolConversations, conversations[i])
			}

			reasoning := firstToolCallReasoning(toolConversations)
			if reasoning == "" {
				reasoning = pendingToolReasoning
			}
			pendingToolReasoning = ""

			chatMessages = append(chatMessages, o.convertToolCallAssistantMessage(toolConversations, pendingToolContent, reasoning))
			pendingToolContent = ""
			for _, toolConversation := range toolConversations {
				chatMessages = append(chatMessages, openAIChatMessage{
					Role:       "tool",
					Content:    stringPtr(toolConversation.ToolCallInfo.Response),
					ToolCallID: toolConversation.ToolCallInfo.Id,
				})
			}
		}
	}

	return chatMessages, nil
}

// shouldFoldAssistantIntoNextToolCall keeps DeepSeek's content, reasoning, and tool calls in one assistant message.
func (o *OpenAIBaseProvider) shouldFoldAssistantIntoNextToolCall(conversations []common.Conversation, index int) bool {
	if !o.shouldReplayToolReasoningContent() {
		return false
	}
	return index+1 < len(conversations) && conversations[index+1].Role == common.ConversationRoleTool
}

// convertAssistantMessage preserves provider-specific reasoning fields in chat history.
func (o *OpenAIBaseProvider) convertAssistantMessage(content string, reasoning string) openAIChatMessage {
	assistant := openAIChatMessage{
		Role:    "assistant",
		Content: stringPtr(content),
	}
	if o.shouldReplayToolReasoningContent() {
		assistant.ReasoningContent = stringPtr(reasoning)
	}
	return assistant
}

func (o *OpenAIBaseProvider) convertToolCallAssistantMessage(toolConversations []common.Conversation, content string, reasoning string) openAIChatMessage {
	toolCalls := make([]openAIMessageToolCall, 0, len(toolConversations))
	for _, toolConversation := range toolConversations {
		toolCalls = append(toolCalls, openAIMessageToolCall{
			ID:   toolConversation.ToolCallInfo.Id,
			Type: "function",
			Function: openAIMessageToolCallFunc{
				Name:      toolConversation.ToolCallInfo.Name,
				Arguments: toolConversation.ToolCallInfo.Delta,
			},
		})
	}

	assistant := openAIChatMessage{
		Role:      "assistant",
		ToolCalls: toolCalls,
	}
	if o.shouldReplayToolReasoningContent() {
		// DeepSeek V4 thinking mode requires reasoning_content to be replayed
		// on assistant messages that contain tool_calls.
		assistant.Content = stringPtr(content)
		assistant.ReasoningContent = stringPtr(reasoning)
	}

	return assistant
}

func firstToolCallReasoning(toolConversations []common.Conversation) string {
	for _, toolConversation := range toolConversations {
		if toolConversation.Reasoning != "" {
			return toolConversation.Reasoning
		}
	}
	return ""
}

func (o *OpenAIBaseProvider) shouldReplayToolReasoningContent() bool {
	providerName := strings.ToLower(string(o.connectContext.Name))
	providerHost := strings.ToLower(o.connectContext.Host)
	return providerName == "deepseek" || strings.Contains(providerHost, "deepseek.com")
}

// chatImageURL preserves supported encoded images and converts other local raster formats to PNG.
func chatImageURL(source common.WoxImage) (string, error) {
	switch source.ImageType {
	case common.WoxImageTypeUrl:
		if !strings.HasPrefix(source.ImageData, "https://") && !strings.HasPrefix(source.ImageData, "http://") {
			return "", fmt.Errorf("invalid image URL")
		}
		return source.ImageData, nil
	case common.WoxImageTypeBase64:
		if !strings.HasPrefix(source.ImageData, "data:image/") {
			return "", fmt.Errorf("invalid image data URL")
		}
		return source.ImageData, nil
	case common.WoxImageTypeAbsolutePath:
		data, err := common.ReadChatImage(source.ImageData)
		if err != nil {
			return "", err
		}
		mimeType := http.DetectContentType(data)
		switch mimeType {
		case "image/png", "image/jpeg", "image/gif", "image/webp":
			return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
		}
		decoded, err := source.ToImageWithoutRemoteFetch()
		if err != nil {
			return "", err
		}
		encoded, err := common.NewWoxImage(decoded)
		if err != nil {
			return "", err
		}
		return encoded.ImageData, nil
	default:
		return "", fmt.Errorf("unsupported chat image type: %s", source.ImageType)
	}
}
