package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"wox/common"
	"wox/setting"
	"wox/util"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/pagination"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/packages/ssestream"
)

type OpenAIBaseProviderOptions struct {
	Headers            map[string]string
	ChatRequestOptions func(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions) []option.RequestOption
}

// OpenAIBaseProvider is the base provider for all OpenAI compatible providers
type OpenAIBaseProvider struct {
	connectContext setting.AIProvider
	options        OpenAIBaseProviderOptions
}

// OpenAIBaseProviderStream represents a stream from OpenAI compatible providers
type OpenAIBaseProviderStream struct {
	stream            *ssestream.Stream[openai.ChatCompletionChunk]
	conversations     []common.Conversation
	acc               openai.ChatCompletionAccumulator
	accumulatedReason string // accumulated reasoning content from chunks
	accumulatedData   string // accumulated answer content after provider-specific reasoning markers are removed
	pendingData       string // buffered content that may contain a split content tag marker across stream chunks
	activeContentTag  streamContentTag
	hasContentTag     bool // true when content chunks are currently inside a configured tagged block
	trimAnswerPrefix  bool // true after a tagged block so separator whitespace is not kept as answer text
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
	client := o.getClient(ctx)
	requestOptions := o.getChatRequestOptions(ctx, model, conversations, options)

	util.GetLogger().Debug(ctx, fmt.Sprintf("AI: chat stream with model: %s, conversations: %d, tools: %d", model.Name, len(conversations), len(options.Tools)))

	for i, conv := range conversations {
		util.GetLogger().Debug(ctx, fmt.Sprintf("AI: conversation[%d] - role: %s, text: %s, toolCallID: %s", i, conv.Role, conv.Text, conv.ToolCallInfo.Id))
	}
	convertedTools := o.convertTools(options.Tools)
	for i, tool := range convertedTools {
		if function := tool.GetFunction(); function != nil {
			util.GetLogger().Debug(ctx, fmt.Sprintf("AI: converted tool[%d] name: %s, paramters: %v", i, function.Name, function.Parameters))
		}
	}

	var createdStream *ssestream.Stream[openai.ChatCompletionChunk]
	if len(options.Tools) > 0 {
		chatParams := openai.ChatCompletionNewParams{
			Model:    model.Name,
			Messages: o.convertConversations(conversations),
			Tools:    convertedTools,
			ToolChoice: openai.ChatCompletionToolChoiceOptionUnionParam{
				OfAuto: param.Opt[string]{},
			},
		}
		createdStream = client.Chat.Completions.NewStreaming(ctx, chatParams, requestOptions...)
	} else {
		createdStream = client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
			Model:    model.Name,
			Messages: o.convertConversations(conversations),
		}, requestOptions...)
	}

	return &OpenAIBaseProviderStream{conversations: conversations, stream: createdStream}, nil
}

func (o *OpenAIBaseProvider) getChatRequestOptions(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions) []option.RequestOption {
	if o.options.ChatRequestOptions == nil {
		return nil
	}

	return o.options.ChatRequestOptions(ctx, model, conversations, options)
}

// Models returns the list of available models from the OpenAI compatible provider
func (o *OpenAIBaseProvider) Models(ctx context.Context) ([]common.Model, error) {
	client := o.getClient(ctx)
	models, err := client.Models.List(ctx)
	if err != nil {
		return nil, err
	}

	pageAutoPager := pagination.NewPageAutoPager(models, err)
	var openaiModels []common.Model
	for pageAutoPager.Next() {
		model := pageAutoPager.Current()
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
	client := o.getClient(ctx)
	_, err := client.Models.List(ctx)
	return err
}

func (o *OpenAIBaseProvider) convertTools(tools []common.Tool) []openai.ChatCompletionToolUnionParam {
	/*
		{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        "getCurrentWeather",
				Description: "Get the current weather in a given location",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"rationale": {
							Type:        jsonschema.String,
							Description: "The rationale for choosing this function call with these parameters",
						},
						"location": {
							Type:        jsonschema.String,
							Description: "The city and state, e.g. San Francisco, CA",
						},
						"unit": {
							Type: jsonschema.String,
							Enum: []string{"celsius", "fahrenheit"},
						},
					},
					Required: []string{"rationale", "location"},
				},
			},
		}
	*/
	convertedTools := make([]openai.ChatCompletionToolUnionParam, len(tools))
	for i, tool := range tools {
		parametersMap := make(map[string]any)
		parametersMap["type"] = tool.Parameters.Type

		if tool.Parameters.Properties != nil {
			parametersMap["properties"] = tool.Parameters.Properties
		} else {
			parametersMap["properties"] = map[string]any{}
		}

		if len(tool.Parameters.Required) > 0 {
			parametersMap["required"] = tool.Parameters.Required
		}

		convertedTools[i] = openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        tool.Name,
			Description: openai.String(tool.Description),
			Parameters:  openai.FunctionParameters(parametersMap),
		})
	}
	return convertedTools
}

func (s *OpenAIBaseProviderStream) Receive(ctx context.Context) (common.ChatStreamData, error) {
	if !s.stream.Next() {
		if s.stream.Err() != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("AI: Stream error: %v", s.stream.Err()))
			return common.ChatStreamData{}, s.stream.Err()
		}

		var toolCallInfos []common.ToolCallInfo
		if len(s.acc.Choices) > 0 && len(s.acc.Choices[0].Message.ToolCalls) > 0 {
			toolCalls := s.acc.Choices[0].Message.ToolCalls
			util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Tool call streaming finished, tool calls count: %d", len(toolCalls)))

			for _, toolCall := range toolCalls {
				toolCallInfo := common.ToolCallInfo{
					Id:    toolCall.ID,
					Name:  toolCall.Function.Name,
					Delta: toolCall.Function.Arguments,
				}

				// try to unmarshal tool call arguments if possible
				var argsMap map[string]any
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &argsMap); err == nil {
					toolCallInfo.Arguments = s.normalizeArguments(ctx, toolCall.Function.Name, argsMap)
					toolCallInfo.Status = common.ToolCallStatusPending
				} else {
					util.GetLogger().Error(ctx, fmt.Sprintf("AI: Failed to unmarshal tool call arguments, json=%s, err: %s", toolCall.Function.Arguments, err.Error()))
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

	chunk := s.stream.Current()
	util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Received raw chunk: %s", chunk.RawJSON()))

	// Store previous normalized content and reasoning before adding chunk so
	// tag-only chunks do not look like user-visible answer updates.
	previousContent := s.accumulatedData
	var previousReasoning string
	previousReasoning = s.accumulatedReason

	// Extract reasoning from current chunk if present
	if len(chunk.Choices) > 0 {
		delta := chunk.Choices[0].Delta

		for _, reasoningFieldName := range reasoningExtraFieldNames {
			if reasoningField, exists := delta.JSON.ExtraFields[reasoningFieldName]; exists {
				// DeepSeek streams thinking as reasoning_content while some other
				// OpenAI-compatible providers use reasoning. The old single-field
				// branch dropped DeepSeek thinking chunks as empty stream updates,
				// so keep the compatible field names together and parse them through
				// the same path.
				rawReasoning := reasoningField.Raw()

				// Only process if reasoning is not null.
				if rawReasoning != "null" && rawReasoning != "" {
					var reasoningStr string
					if err := json.Unmarshal([]byte(rawReasoning), &reasoningStr); err == nil {
						if reasoningStr != "" {
							s.accumulatedReason += reasoningStr
							util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Extracted reasoning from chunk field %s: %s", reasoningFieldName, reasoningStr))
						}
					} else {
						util.GetLogger().Error(ctx, fmt.Sprintf("AI: Failed to unmarshal reasoning field %s: %s, error: %s", reasoningFieldName, rawReasoning, err.Error()))
					}
				}
			}
		}

		if delta.Content != "" {
			s.appendContentDelta(delta.Content)
		}
	}

	s.acc.AddChunk(chunk)

	// Check if normalized answer content has changed after processing the chunk.
	// The OpenAI SDK accumulator still keeps the raw content for protocol state,
	// but UI callers should receive content with configured content tags removed.
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
	var totalToolCallCount = len(s.acc.Choices[0].Message.ToolCalls)
	if totalToolCallCount > 0 {
		var toolCallInfos []common.ToolCallInfo
		for index, toolcall := range s.acc.Choices[0].Message.ToolCalls {
			isLastToolCall := index == totalToolCallCount-1

			// if the toolcall is not the last one, we will set the status to pending, because the tool call streaming is one by one
			// the prev toolcall streaming must be finished before the next one
			status := common.ToolCallStatusStreaming
			if totalToolCallCount > 1 && !isLastToolCall {
				status = common.ToolCallStatusPending
			}

			toolCallInfo := common.ToolCallInfo{
				Id:        toolcall.ID,
				Name:      toolcall.Function.Name,
				Arguments: map[string]any{},
				Delta:     toolcall.Function.Arguments,
				Status:    status,
			}
			toolCallInfos = append(toolCallInfos, toolCallInfo)
		}
		streamData.ToolCalls = toolCallInfos
	}

	return streamData, nil
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

func (s *OpenAIBaseProviderStream) isChunkEmpty(chunk openai.ChatCompletionChunk) bool {
	if len(chunk.Choices) == 0 {
		return true
	}

	delta := chunk.Choices[0].Delta

	// Check regular fields
	if delta.Content != "" || delta.Refusal != "" || len(delta.ToolCalls) > 0 {
		return false
	}

	// Check for reasoning fields in ExtraFields. DeepSeek uses reasoning_content,
	// while other OpenAI-compatible providers use reasoning, and both should
	// keep streaming chunks alive even when there is no user-visible content.
	for _, reasoningFieldName := range reasoningExtraFieldNames {
		if reasoningField, exists := delta.JSON.ExtraFields[reasoningFieldName]; exists && reasoningField.Valid() {
			return false
		}
	}

	return true
}

// convertConversations converts the conversations to OpenAI format
func (o *OpenAIBaseProvider) convertConversations(conversations []common.Conversation) []openai.ChatCompletionMessageParamUnion {
	var chatMessages []openai.ChatCompletionMessageParamUnion
	pendingToolReasoning := ""
	for i := 0; i < len(conversations); i++ {
		conversation := conversations[i]
		if conversation.Role == common.ConversationRoleSystem {
			chatMessages = append(chatMessages, openai.SystemMessage(conversation.Text))
		}
		if conversation.Role == common.ConversationRoleUser {
			chatMessages = append(chatMessages, openai.UserMessage(conversation.Text))
		}
		if conversation.Role == common.ConversationRoleAssistant {
			if o.shouldFoldReasoningIntoNextToolCall(conversations, i) {
				pendingToolReasoning = conversation.Reasoning
				continue
			}
			chatMessages = append(chatMessages, openai.AssistantMessage(conversation.Text))
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

			chatMessages = append(chatMessages, o.convertToolCallAssistantMessage(toolConversations, reasoning))
			for _, toolConversation := range toolConversations {
				chatMessages = append(chatMessages, openai.ToolMessage(toolConversation.ToolCallInfo.Response, toolConversation.ToolCallInfo.Id))
			}
		}
	}

	return chatMessages
}

func (o *OpenAIBaseProvider) shouldFoldReasoningIntoNextToolCall(conversations []common.Conversation, index int) bool {
	if !o.shouldReplayToolReasoningContent() {
		return false
	}
	conversation := conversations[index]
	return conversation.Text == "" && conversation.Reasoning != "" && index+1 < len(conversations) && conversations[index+1].Role == common.ConversationRoleTool
}

func (o *OpenAIBaseProvider) convertToolCallAssistantMessage(toolConversations []common.Conversation, reasoning string) openai.ChatCompletionMessageParamUnion {
	toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, 0, len(toolConversations))
	for _, toolConversation := range toolConversations {
		toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
			OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
				ID: toolConversation.ToolCallInfo.Id,
				Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
					Name:      toolConversation.ToolCallInfo.Name,
					Arguments: toolConversation.ToolCallInfo.Delta,
				},
			},
		})
	}

	assistant := openai.ChatCompletionAssistantMessageParam{ToolCalls: toolCalls}
	if o.shouldReplayToolReasoningContent() {
		// DeepSeek V4 thinking mode requires reasoning_content to be replayed
		// on assistant messages that contain tool_calls. The OpenAI-compatible
		// SDK has no typed field for this provider extension, so use extras.
		assistant.SetExtraFields(map[string]any{"reasoning_content": reasoning})
	}

	return openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant}
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

// getClient returns an OpenAI client
func (o *OpenAIBaseProvider) getClient(ctx context.Context) openai.Client {
	var requestOption = []option.RequestOption{
		option.WithBaseURL(o.connectContext.Host),
		option.WithAPIKey(o.connectContext.ApiKey),
		// Some OpenAI-compatible relays block SDK fingerprint headers even when the same bearer token works with curl. #4473
		option.WithHTTPClient(util.GetHTTPClient(ctx)),
		option.WithHeader("User-Agent", "Wox"),
		option.WithHeaderDel("X-Stainless-Lang"),
		option.WithHeaderDel("X-Stainless-Package-Version"),
		option.WithHeaderDel("X-Stainless-OS"),
		option.WithHeaderDel("X-Stainless-Arch"),
		option.WithHeaderDel("X-Stainless-Runtime"),
		option.WithHeaderDel("X-Stainless-Runtime-Version"),
		option.WithHeaderDel("X-Stainless-Retry-Count"),
		option.WithHeaderDel("X-Stainless-Timeout"),
	}

	// with custom headers
	if o.options.Headers != nil {
		for k, v := range o.options.Headers {
			requestOption = append(requestOption, option.WithHeaderAdd(k, v))
		}
	}

	return openai.NewClient(requestOption...)
}
