package launcher

import "testing"

func TestGridSelectionIndexFollowsVisualRows(t *testing.T) {
	results := []queryResult{{IsGroup: true}}
	for range 16 {
		results = append(results, queryResult{})
	}
	results = append(results, queryResult{IsGroup: true})
	for range 10 {
		results = append(results, queryResult{})
	}

	tests := []struct {
		name      string
		current   int
		direction int
		want      int
	}{
		{name: "down clamps to short row", current: 10, direction: 1, want: 16},
		{name: "down crosses group at same column", current: 11, direction: 1, want: 18},
		{name: "up wraps to last row", current: 1, direction: -1, want: 18},
		{name: "down wraps to first row", current: 27, direction: 1, want: 10},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := gridSelectionIndex(results, test.current, 10, test.direction); got != test.want {
				t.Fatalf("gridSelectionIndex() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestVisibleGridResultsIncludesViewportOverscan(t *testing.T) {
	results := []queryResult{{IsGroup: true}}
	for range 50 {
		results = append(results, queryResult{})
	}

	visible := visibleGridResults(results, 10, 40, 72, 40)
	for index := 1; index <= 50; index++ {
		want := index <= 40
		if visible[index] != want {
			t.Fatalf("visible[%d] = %t, want %t", index, visible[index], want)
		}
	}
}

func TestGridResultVerticalBoundsIncludesHeaderForFirstGroupRow(t *testing.T) {
	results := []queryResult{{IsGroup: true}, {}, {}, {}, {}, {}, {}}
	layout := &gridLayout{Columns: 3, AspectRatio: 1}

	firstTop, firstBottom := gridResultVerticalBounds(results, 3, 328, layout)
	if firstTop != 0 || firstBottom != 132 {
		t.Fatalf("first row bounds = (%v, %v), want (0, 132)", firstTop, firstBottom)
	}
	secondTop, secondBottom := gridResultVerticalBounds(results, 4, 328, layout)
	if secondTop != 132 || secondBottom != 232 {
		t.Fatalf("second row bounds = (%v, %v), want (132, 232)", secondTop, secondBottom)
	}
}
