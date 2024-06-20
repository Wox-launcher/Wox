package ai

type ConversationRole string

var (
	ConversationRoleUser   ConversationRole = "user"
	ConversationRoleSystem ConversationRole = "system"
)

type Conversation struct {
	Role      ConversationRole
	Text      string
	Timestamp int64
}
