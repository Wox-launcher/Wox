package selection

import "context"

func GetSelected(ctx context.Context) (Selection, error) {
	return getSelectedByClipboard(ctx)
}
