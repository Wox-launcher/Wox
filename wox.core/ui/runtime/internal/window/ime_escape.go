package window

import "time"

// imeEscapeGuard follows one physical Escape across Windows IME and key messages.
// TSF can end the composition before it delivers the corresponding key down.
type imeEscapeGuard struct {
	compositionEnded time.Time
	held             bool
}

func (g *imeEscapeGuard) consume(down, repeat, composing bool, now time.Time) bool {
	if !down {
		held := g.held
		*g = imeEscapeGuard{}
		return held
	}
	recentlyEnded := !g.compositionEnded.IsZero() && now.Sub(g.compositionEnded) <= 35*time.Millisecond
	g.compositionEnded = time.Time{}
	g.held = composing || recentlyEnded || (repeat && g.held)
	return g.held
}
