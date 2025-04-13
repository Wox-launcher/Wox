package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAICommandParseThinking(t *testing.T) {
	thinking, content := processAIThinking("<think> hello world</think>this is content")
	assert.Equal(t, " hello world", thinking)
	assert.Equal(t, "this is content", content)

	thinking, content = processAIThinking("\n<think> hello world</think>this is content")
	assert.Equal(t, " hello world", thinking)
	assert.Equal(t, "this is content", content)

	thinking, content = processAIThinking("hello world")
	assert.Equal(t, "", thinking)
	assert.Equal(t, "hello world", content)

	thinking, content = processAIThinking("think is in the middle of the text<think> should not be included")
	assert.Equal(t, "", thinking)
	assert.Equal(t, "think is in the middle of the text<think> should not be included", content)

	thinking, content = processAIThinking("<think> think is not end with think should be included")
	assert.Equal(t, " think is not end with think should be included", thinking)
	assert.Equal(t, "", content)

	thinking, content = processAIThinking("<think>think is in the middle of the text</think> should not <think> be included")
	assert.Equal(t, "think is in the middle of the text", thinking)
	assert.Equal(t, " should not <think> be included", content)
}
