package widget

import (
	"testing"
	"time"

	woxui "wox/ui/runtime"
)

type recordingHostServices struct {
	fakeHostServices
	work woxui.FrameWorkMetrics
}

func (*recordingHostServices) RecordFramePhase(uint64, woxui.FrameMetricPhase, time.Duration) {}

func (*recordingHostServices) RecordFrameCounts(uint64, int, int, int, woxui.Rect) {}

func (r *recordingHostServices) RecordFrameWork(_ uint64, work woxui.FrameWorkMetrics) {
	r.work = work
}

func TestHostRecordsBoundaryReuseAsSingleLayoutVisit(t *testing.T) {
	builds := 0
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Flex{Axis: Vertical, Children: []Widget{
			Text{Value: "stable", Style: woxui.TextStyle{Size: 12}},
			Boundary[boundaryTestProps]{Key: "cached", Props: boundaryTestProps{Value: 1}, Build: func(boundaryTestProps) Widget {
				builds++
				return Flex{Axis: Vertical, Children: []Widget{
					Text{Value: "one", Style: woxui.TextStyle{Size: 12}},
					Text{Value: "two", Style: woxui.TextStyle{Size: 12}},
					Text{Value: "three", Style: woxui.TextStyle{Size: 12}},
				}}
			}},
		}}
	})
	services := &recordingHostServices{}
	host.AttachServices(services)

	first := &woxui.DisplayList{}
	first.AttachFrameMetricsID(1)
	host.Frame(first, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 100}})
	if builds != 1 || services.work.BoundaryBuilds != 1 || services.work.LayoutVisits < 4 {
		t.Fatalf("first frame work = builds %d %+v", builds, services.work)
	}
	firstLayout := services.work.LayoutVisits

	second := &woxui.DisplayList{}
	second.AttachFrameMetricsID(2)
	host.Frame(second, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 100}})
	if builds != 1 || services.work.BoundaryBuilds != 0 || services.work.BoundaryReuses != 1 {
		t.Fatalf("cached frame work = builds %d %+v", builds, services.work)
	}
	if services.work.LayoutVisits >= firstLayout {
		t.Fatalf("cached layout visits = %d, want fewer than first-frame %d", services.work.LayoutVisits, firstLayout)
	}
	if services.work.TextDraws == 0 || services.work.PaintVisits == 0 || services.work.IdentityVisits == 0 || services.work.A11yVisits == 0 {
		t.Fatalf("cached frame missing visit counters: %+v", services.work)
	}
}
