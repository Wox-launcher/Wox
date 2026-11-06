package woxview

import (
	"fmt"
	"slices"
	"time"

	ui "wox/ui/runtime"
	"wox/ui/widget"
)

type TimerDirection string

const (
	TimerUp   TimerDirection = "up"
	TimerDown TimerDirection = "down"
)

func (TimerDirection) enumValues() []string { return []string{"", "up", "down"} }

type TimerFormat string

const (
	TimerFormatHours                     TimerFormat = "HH"
	TimerFormatMinutes                   TimerFormat = "MM"
	TimerFormatSeconds                   TimerFormat = "SS"
	TimerFormatHoursMinutes              TimerFormat = "HH:MM"
	TimerFormatMinutesSeconds            TimerFormat = "MM:SS"
	TimerFormatHoursMinutesSeconds       TimerFormat = "HH:MM:SS"
	TimerFormatSecondsMillis             TimerFormat = "SS.SSS"
	TimerFormatMinutesSecondsMillis      TimerFormat = "MM:SS.SSS"
	TimerFormatHoursMinutesSecondsMillis TimerFormat = "HH:MM:SS.SSS"
)

func (TimerFormat) enumValues() []string {
	return []string{"", "HH", "MM", "SS", "HH:MM", "MM:SS", "HH:MM:SS", "SS.SSS", "MM:SS.SSS", "HH:MM:SS.SSS"}
}

const maxTimerMilliseconds int64 = 365 * 24 * 60 * 60 * 1000

// TimerProps describes a stopwatch or bounded timer driven by the native clock.
// Elapsed is time already consumed in milliseconds, independent of direction.
type TimerProps struct {
	Styled
	Elapsed    int64          `json:"elapsed,omitempty"`
	Duration   *int64         `json:"duration,omitempty"`
	Direction  TimerDirection `json:"direction,omitempty"`
	Format     TimerFormat    `json:"format,omitempty"`
	Running    bool           `json:"running,omitempty"`
	Paused     bool           `json:"paused,omitempty"`
	PlaybackID string         `json:"playbackId,omitempty"`
	Animate    bool           `json:"animate,omitempty"`
	OnComplete *Handler       `json:"onComplete,omitempty"`
}

func (p *TimerProps) handlers() []namedHandler {
	return append(p.Styled.handlers(), namedHandler{"onComplete", p.OnComplete})
}

// prepare rejects ambiguous countdowns and bounds all native time conversions.
func (p *TimerProps) prepare() error {
	if p.Elapsed < 0 || p.Elapsed > maxTimerMilliseconds || len(p.PlaybackID) > 256 {
		return fmt.Errorf("invalid timer elapsed or playbackId")
	}
	if !slices.Contains(p.Direction.enumValues(), string(p.Direction)) || !slices.Contains(p.Format.enumValues(), string(p.Format)) {
		return fmt.Errorf("invalid timer direction or format")
	}
	if p.Duration != nil && (*p.Duration < 0 || *p.Duration > maxTimerMilliseconds) {
		return fmt.Errorf("timer.duration must be between 0 and %d milliseconds", maxTimerMilliseconds)
	}
	if (p.Direction == TimerDown || p.OnComplete != nil) && p.Duration == nil {
		return fmt.Errorf("timer.duration is required for countdown or onComplete")
	}
	return p.Style.validate()
}

// timerText uses cumulative leading units; countdowns round up so they never
// display zero before their deadline. SSS after a decimal point is milliseconds.
func timerText(value time.Duration, format TimerFormat, countdown bool) string {
	precision := time.Second
	switch format {
	case TimerFormatHours:
		precision = time.Hour
	case TimerFormatMinutes, TimerFormatHoursMinutes:
		precision = time.Minute
	case TimerFormatSecondsMillis, TimerFormatMinutesSecondsMillis, TimerFormatHoursMinutesSecondsMillis:
		precision = time.Millisecond
	}
	value = max(0, value)
	if countdown && value%precision != 0 {
		value += precision - value%precision
	}
	seconds := int64(value / time.Second)
	hours, minutes, millis := seconds/3600, seconds/60, int64(value/time.Millisecond)%1000
	switch format {
	case TimerFormatHours:
		return fmt.Sprintf("%02d", hours)
	case TimerFormatMinutes:
		return fmt.Sprintf("%02d", minutes)
	case TimerFormatSeconds:
		return fmt.Sprintf("%02d", seconds)
	case TimerFormatHoursMinutes:
		return fmt.Sprintf("%02d:%02d", hours, minutes%60)
	case TimerFormatHoursMinutesSeconds:
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes%60, seconds%60)
	case TimerFormatSecondsMillis:
		return fmt.Sprintf("%02d.%03d", seconds, millis)
	case TimerFormatMinutesSecondsMillis:
		return fmt.Sprintf("%02d:%02d.%03d", minutes, seconds%60, millis)
	case TimerFormatHoursMinutesSecondsMillis:
		return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes%60, seconds%60, millis)
	default:
		return fmt.Sprintf("%02d:%02d", minutes, seconds%60)
	}
}

type timerConfig struct {
	builder builder
	props   *TimerProps
}

// timerPlayback excludes display settings and elapsed echoes, which must not
// restart a running clock or rearm its completion callback.
type timerPlayback struct {
	id        string
	direction TimerDirection
	duration  int64
	bounded   bool
}

type timerState struct {
	playback  timerPlayback
	completed bool
	value     string
	changedAt []time.Duration
}

func (s *timerState) InitState(_ widget.StateContext, value any) {
	s.receive(value.(timerConfig).props)
}
func (*timerState) Dispose() {}
func (s *timerState) DidUpdateWidget(_ widget.StateContext, _, next any) {
	s.receive(next.(timerConfig).props)
}

// playback normalizes defaults so explicit defaults do not restart the clock.
func (p *TimerProps) playback() timerPlayback {
	playback := timerPlayback{id: p.PlaybackID, direction: p.Direction, bounded: p.Duration != nil}
	if playback.direction == "" {
		playback.direction = TimerUp
	}
	if p.Duration != nil {
		playback.duration = *p.Duration
	}
	return playback
}

// receive resets only when the consumer starts a distinct playback.
func (s *timerState) receive(p *TimerProps) {
	playback := p.playback()
	if playback != s.playback {
		s.playback, s.completed, s.value, s.changedAt = playback, false, "", nil
	}
}

// Build keeps pause/resume and formatting independent of producer update cadence.
func (s *timerState) Build(ctx widget.StateContext, value any) widget.Widget {
	c := value.(timerConfig)
	p := c.props
	build := func(elapsed time.Duration) widget.Widget {
		complete := p.Duration != nil && elapsed >= time.Duration(*p.Duration)*time.Millisecond
		if p.Running && !p.Paused && complete && !s.completed {
			s.completed = true
			if callback := c.builder.tap(p.OnComplete); callback != nil {
				playback := s.playback
				ctx.PostFrame(func() {
					// An earlier post-frame callback can replace this playback before
					// its event is delivered, even if the node and handler stay keyed.
					current := c.builder.nodeOwner.node.Props.(*TimerProps)
					if ctx.Mounted() && current.playback() == playback && current.Running && !current.Paused && current.OnComplete != nil && current.OnComplete.ID == p.OnComplete.ID {
						callback()
					} else if s.playback == playback {
						s.completed = false
					}
				})
			}
		}
		return s.display(c, elapsed, complete)
	}
	if !p.Running || p.Duration != nil && *p.Duration == 0 {
		return build(time.Duration(p.Elapsed) * time.Millisecond)
	}
	duration := time.Duration(0)
	if p.Duration != nil {
		duration = time.Duration(*p.Duration) * time.Millisecond
	}
	return widget.TimelineAnimation{Key: widget.Key(fmt.Sprintf("%p/%s/timer-clock", c.builder.view, c.builder.nodeOwner.stateKey)), PlaybackID: string(s.playback.direction) + "/" + p.PlaybackID, Duration: duration, Elapsed: time.Duration(p.Elapsed) * time.Millisecond, Paused: p.Paused, Builder: build}
}

// display animates changed digits in either direction while keeping tabular widths.
func (s *timerState) display(c timerConfig, elapsed time.Duration, complete bool) widget.Widget {
	p := c.props
	amount := elapsed
	if p.Duration != nil {
		duration := time.Duration(*p.Duration) * time.Millisecond
		amount = min(amount, duration)
		if p.Direction == TimerDown {
			amount = max(0, duration-elapsed)
		}
	}
	value := timerText(amount, p.Format, p.Direction == TimerDown)
	if len(value) != len(s.value) {
		s.changedAt = make([]time.Duration, len(value))
		for i := range s.changedAt {
			s.changedAt[i] = elapsed - time.Second
		}
	} else {
		for i := range value {
			if value[i] != s.value[i] {
				s.changedAt[i] = elapsed
			}
		}
	}
	s.value = value
	theme := c.builder.theme()
	style := p.Style.resolve(theme)
	textStyle := style.textStyle(theme, 16)
	if style.FontFamily == "" {
		textStyle.Family = ui.FontFamilyMonospace
	}
	color := style.Color.resolve(theme, theme.Text)
	children := make([]widget.Widget, 0, len(value))
	for index, digit := range value {
		width := textStyle.Size * .66
		var glyph widget.Widget = widget.Text{Value: string(digit), Style: textStyle, Color: color}
		if digit == ':' || digit == '.' {
			width = textStyle.Size * .5
		} else if p.Running && !p.Paused && p.Animate && !complete {
			phase := min(float32(1), max(float32(0), float32(elapsed-s.changedAt[index])/float32(240*time.Millisecond)))
			glyph = widget.Transform{Matrix: ui.TranslationTransform(0, (1-phase)*7), Child: widget.Opacity{Value: .4 + .6*phase, Child: glyph}}
		}
		children = append(children, widget.Clip{Width: width, Height: textStyle.Size * 1.5, Child: widget.Align{Horizontal: .5, Vertical: .5, Child: glyph}})
	}
	return style.wrapOuter(theme, widget.Flex{Axis: widget.Horizontal, CrossAxisAlignment: widget.CrossAxisCenter, Children: children})
}

func buildTimer(b *builder, node *Node, _ string) widget.Widget {
	return widget.Stateful{Key: b.nodeOwner.stateKey + "/timer", Widget: timerConfig{builder: *b, props: node.Props.(*TimerProps)}, CreateState: func() widget.State { return &timerState{} }}
}
