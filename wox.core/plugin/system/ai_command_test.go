package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAICommandParseThinking(t *testing.T) {

	plugin := &Plugin{}

	thinking, content := plugin.processThinking("<think> hello world</think>this is content")
	assert.Equal(t, " hello world", thinking)
	assert.Equal(t, "this is content", content)

	thinking, content = plugin.processThinking("hello world")
	assert.Equal(t, "", thinking)
	assert.Equal(t, "hello world", content)

	thinking, content = plugin.processThinking("think is in the middle of the text<think> should not be included")
	assert.Equal(t, "", thinking)
	assert.Equal(t, "think is in the middle of the text<think> should not be included", content)

	thinking, content = plugin.processThinking("<think> think is not end with think should be included")
	assert.Equal(t, " think is not end with think should be included", thinking)
	assert.Equal(t, "", content)
}
