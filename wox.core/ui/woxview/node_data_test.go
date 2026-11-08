package woxview

import (
	"bytes"
	"testing"
)

// TestNodeDataUpdatesChartPaint checks that each supported data snapshot can
// reach the painter while the document itself and unrelated nodes stay intact.
func TestNodeDataUpdatesChartPaint(t *testing.T) {
	for _, tc := range []struct{ kind, props, update, invalid, latest string }{
		{"line-chart", `{"values":[0,1],"pointLabels":["A","B"]}`, `{"values":[1,0]}`, `{"values":[1]}`, `{"values":[0,1]}`},
		{"scatter-chart", `{"points":[{"x":0,"y":1},{"x":1,"y":0}],"pointRadii":[2,0]}`, `{"pointRadii":[0,3]}`, `{"pointRadii":[1,17]}`, `{"points":[{"x":0,"y":0},{"x":1,"y":1}]}`},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			doc := `{"version":1,"root":{"type":"column","children":[{"type":"` + tc.kind + `","key":"chart","props":` + tc.props + `}]}}`
			view := NewView()
			initial := mustParse(t, doc)
			view.SetTree(initial)
			var events []recordedEvent
			host := renderView(t, view, &events)
			t.Cleanup(host.Dispose)
			before := animationFrameImage(t, host, 1).Pix
			patch := func(raw string) error {
				data, err := ParseNodeData([]byte(raw))
				if err != nil {
					return err
				}
				return view.UpdateNodeData("chart", data)
			}
			if err := patch(tc.update); err != nil {
				t.Fatal(err)
			}
			after := animationFrameImage(t, host, 1).Pix
			if bytes.Equal(before, after) || view.Tree() != initial {
				t.Fatal("update did not repaint the retained chart independently of the tree")
			}
			if err := patch(tc.invalid); err == nil || !bytes.Equal(after, animationFrameImage(t, host, 1).Pix) {
				t.Fatal("invalid update changed the chart")
			}
			if err := patch(tc.latest); err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(after, animationFrameImage(t, host, 1).Pix) {
				t.Fatal("latest update did not repaint")
			}
			view.SetTree(mustParse(t, doc))
			if !bytes.Equal(before, animationFrameImage(t, host, 1).Pix) {
				t.Fatal("data override survived a full document replacement")
			}
		})
	}
}

// TestNodeDataRejectsAmbiguousOrUnsupportedTargets verifies addressing and payload boundaries.
func TestNodeDataRejectsAmbiguousOrUnsupportedTargets(t *testing.T) {
	view := NewView()
	view.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[
		{"type":"text","key":"label","props":{"text":"old"}},
		{"type":"column","children":[{"type":"line-chart","key":"chart","props":{"values":[0]}}]},
		{"type":"column","children":[{"type":"line-chart","key":"chart","props":{"values":[0]}}]}
	]}}`))
	for _, key := range []string{"chart", "label", "missing", ""} {
		if err := view.UpdateNodeData(key, NodeData{Values: []float64{1}}); err == nil {
			t.Errorf("accepted target %q", key)
		}
	}
	if _, err := ParseNodeData([]byte(`{"style":{"height":80}}`)); err == nil {
		t.Fatal("accepted a non-data field")
	}
}
