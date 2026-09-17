package ui

import (
	"context"
	"io"
	"testing"
	"wox/ai"
	"wox/common"
)

type themeSuggestionStream struct{ calls int }

func (s *themeSuggestionStream) Receive(context.Context) (common.ChatStreamData, error) {
	s.calls++
	switch s.calls {
	case 1:
		return common.ChatStreamData{}, ai.ChatStreamNoContentErr
	case 2:
		return common.ChatStreamData{Status: common.ChatStreamStatusStreaming, Reasoning: "Use rounded corners."}, nil
	case 3:
		return common.ChatStreamData{Status: common.ChatStreamStatusStreaming, Data: `{"AppBorderRadius":"24"}`}, nil
	case 4:
		return common.ChatStreamData{Status: common.ChatStreamStatusStreamed, Data: `{"AppBorderRadius":"24"}`}, nil
	default:
		return common.ChatStreamData{}, io.EOF
	}
}

// TestThemeSuggestionSkipsEmptyChunks covers role-only and keepalive chunks before model content.
func TestThemeSuggestionSkipsEmptyChunks(t *testing.T) {
	stream := &themeSuggestionStream{}
	var reasoning string
	result, err := readThemeSuggestion(context.Background(), stream, func(chunk common.ChatStreamData) {
		if chunk.Reasoning != "" {
			reasoning = chunk.Reasoning
		}
	})
	if reasoning != "Use rounded corners." {
		t.Fatal("reasoning-only chunks must reach the UI")
	}
	if err != nil || result != `{"AppBorderRadius":"24"}` || stream.calls != 4 {
		t.Fatalf("result=%q calls=%d err=%v", result, stream.calls, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readThemeSuggestion(ctx, &themeSuggestionStream{}, nil); err != context.Canceled {
		t.Fatalf("cancel error=%v", err)
	}
}
