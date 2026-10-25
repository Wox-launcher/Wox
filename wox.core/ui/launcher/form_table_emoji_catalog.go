package launcher

import (
	"context"
	"sync"

	"wox/util/emojisearch"
)

// formTableEmojiCatalog shares search data and recent choices across table editors.
// Each launcher instance owns its catalog independently of the active editor window.
type formTableEmojiCatalog struct {
	lifecycleCtx                context.Context
	recentFormTableEmojis       []string
	formTableEmojiSearchOnce    sync.Once
	formTableEmojiSearchEntries []emojisearch.Entry
}
