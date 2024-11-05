package ai

import (
	"image"
)

type ConversationRole string

var (
	ConversationRoleUser ConversationRole = "user"
	ConversationRoleAI   ConversationRole = "ai"
)

type Conversation struct {
	Role      ConversationRole
	Text      string
	Images    []image.Image // png images
	Timestamp int64
}
