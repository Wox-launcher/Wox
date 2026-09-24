package preview

// ChatScrollState belongs to the shared chat component. Its zero value follows new
// output; applications retain it with the conversation and pass a copy to the view.
type ChatScrollState struct {
	offset float32
	limit  float32
	paused bool
}

// Position resolves follow mode against the current content, including resize/reflow.
func (s ChatScrollState) Position(limit float32) float32 {
	if !s.paused {
		return max(float32(0), limit)
	}
	return min(max(float32(0), limit), max(float32(0), s.offset))
}

// SetExtent updates keyboard-scroll bounds without treating streaming growth as user input.
func (s *ChatScrollState) SetExtent(limit float32) { s.limit = max(float32(0), limit) }

// chatScrollFollowSlack is how close to the latest message still counts as following it.
const chatScrollFollowSlack = float32(36)

// Scroll records manual scrollback and resumes following near the latest message.
func (s *ChatScrollState) Scroll(delta, limit float32) {
	s.SetExtent(limit)
	s.offset = min(s.limit, max(float32(0), s.Position(s.limit)+delta))
	distance := s.limit - s.offset
	// The slack resumes follow after the user returns near the latest message.
	// When the whole overflow sits inside that slack, distance can never exceed
	// it, so an upward scroll would snap back and hide the start of the conversation.
	s.paused = distance > chatScrollFollowSlack || (s.limit <= chatScrollFollowSlack && s.offset < s.limit)
}

func (s *ChatScrollState) ScrollPage(delta float32) { s.Scroll(delta, s.limit) }
func (s *ChatScrollState) FollowLatest()            { s.paused = false }
