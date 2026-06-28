package ui

import "testing"

func TestLayoutEngineBasic(t *testing.T) {
	engine := &LayoutEngine{
		Theme:    DefaultTheme(),
		Measurer: NewStubMeasurer(),
	}

	// Build a minimal launcher widget tree:
	// VBox { TextBox(query), ListBox(results) }
	root := VBox{
		Padding: 16,
		Gap:     12,
		Children: []Widget{
			TextBox{
				ID:           "query",
				Placeholder:  "Type to search...",
				FontSize:     16,
				FontColor:    ColorTextPrimary,
				BgColor:      RGBA(1, 1, 1, 0.06),
				CornerRadius: 8,
				CursorColor:  ColorCursor,
				Value:        "hello",
				Focused:      true,
				BlinkVisible: true,
			},
			ListBox{
				ID:         "results",
				ItemHeight: 48,
				Items: []ListItem{
					{Title: "Result 1", Subtitle: "Plugin: system"},
					{Title: "Result 2", Subtitle: "Plugin: app"},
					{Title: "Result 3 — 结果3", Subtitle: "Plugin: calculator"},
				},
				Selected:      1,
				SelectedColor: &ColorSelected,
			},
		},
	}

	result := engine.Layout(root, 800, 400)

	if len(result.Commands.Commands) == 0 {
		t.Fatal("expected non-empty command list")
	}

	// Should have at least: rounded rect (textbox bg), text (value),
	// cursor line, selected rect, 3x title text, 3x subtitle text, scrollbar.
	t.Logf("generated %d draw commands", len(result.Commands.Commands))

	// Verify command types are present.
	types := map[CommandType]bool{}
	for _, cmd := range result.Commands.Commands {
		types[cmd.Type] = true
	}

	expectTypes := []CommandType{
		CmdDrawRoundedRect, // textbox background
		CmdDrawText,        // text content
		CmdDrawLine,        // cursor
		CmdPushClip,        // list clip
		CmdDrawImage,       // (no icons in this test, but clip should exist)
		CmdPopClip,         // list clip restore
	}

	for _, et := range expectTypes {
		if et == CmdDrawImage {
			continue // no icons in this test
		}
		if !types[et] {
			t.Errorf("expected command type %d in output", et)
		}
	}
}

func TestCommandListHelpers(t *testing.T) {
	var cl CommandList

	cl.Clear(0, 0, 0, 1)
	cl.DrawRect(0, 0, 100, 50, 1, 0, 0, 1)
	cl.DrawRoundedRect(10, 10, 80, 30, 8, 0, 1, 0, 1)
	cl.DrawText(12, 12, 76, 20, 1, 1, 1, 1, "hi", 14, "")
	cl.PushClip(0, 0, 100, 100)
	cl.PopClip()
	cl.DrawLine(0, 0, 100, 100, 2, 1, 1, 1, 0.5)

	if len(cl.Commands) != 7 {
		t.Fatalf("expected 7 commands, got %d", len(cl.Commands))
	}

	if cl.Commands[0].Type != CmdClear {
		t.Error("first command should be Clear")
	}
	if cl.Commands[2].Type != CmdDrawRoundedRect {
		t.Error("third command should be DrawRoundedRect")
	}
	if cl.Commands[2].Radius != 8 {
		t.Errorf("expected radius 8, got %f", cl.Commands[2].Radius)
	}

	cl.Reset()
	if len(cl.Commands) != 0 {
		t.Error("Reset should clear commands")
	}
}

func TestStubMeasurer(t *testing.T) {
	m := NewStubMeasurer()
	w, h := m.MeasureText("hello", 16, "")
	if w <= 0 || h <= 0 {
		t.Errorf("expected positive dimensions, got w=%f h=%f", w, h)
	}
	// "hello" = 5 chars, each ~0.6*16 = 9.6, total ~48
	if w < 40 || w > 60 {
		t.Errorf("expected width ~48, got %f", w)
	}
}