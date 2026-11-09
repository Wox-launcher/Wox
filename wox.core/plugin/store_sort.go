package plugin

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// StorePluginSort selects how entries are ordered within a catalog.
type StorePluginSort string

const (
	StorePluginSortDefault   StorePluginSort = ""
	StorePluginSortDownloads StorePluginSort = "downloads"
	StorePluginSortName      StorePluginSort = "name"
	StorePluginSortStars     StorePluginSort = "stars"
	StorePluginSortUpdated   StorePluginSort = "updated"
)

const storePluginUpdatedLayout = "2006-01-02 15:04:05"

// StorePluginSortKey holds the catalog and statistics used by both store lists.
type StorePluginSortKey struct {
	Store         string
	Name          string
	StarCount     *int
	DownloadCount *int
	DateUpdated   string
}

func (m StorePluginManifest) SortKey(ctx context.Context) StorePluginSortKey {
	return StorePluginSortKey{
		Store: m.Store, Name: m.GetName(ctx), StarCount: m.StarCount, DownloadCount: m.DownloadCount, DateUpdated: m.DateUpdated,
	}
}

// Less keeps catalog groups together and ranks known statistics ahead of missing ones.
func (order StorePluginSort) Less(left, right StorePluginSortKey) bool {
	leftStore, rightStore := StoreCatalogID(left.Store), StoreCatalogID(right.Store)
	if leftStore != rightStore {
		return StoreCatalogLess(leftStore, rightStore)
	}

	var leftCount, rightCount *int
	switch order {
	case StorePluginSortUpdated:
		leftDate, leftErr := time.Parse(storePluginUpdatedLayout, left.DateUpdated)
		rightDate, rightErr := time.Parse(storePluginUpdatedLayout, right.DateUpdated)
		if (leftErr == nil) != (rightErr == nil) {
			return leftErr == nil
		}
		if leftErr == nil && !leftDate.Equal(rightDate) {
			return leftDate.After(rightDate)
		}
	case StorePluginSortStars:
		leftCount, rightCount = left.StarCount, right.StarCount
	case StorePluginSortName:
	default:
		leftCount, rightCount = left.DownloadCount, right.DownloadCount
	}
	if (leftCount == nil) != (rightCount == nil) {
		return leftCount != nil
	}
	if leftCount != nil && rightCount != nil && *leftCount != *rightCount {
		return *leftCount > *rightCount
	}
	return strings.ToLower(left.Name) < strings.ToLower(right.Name)
}

// MetricLabel formats the statistic used by this sort when the catalog provides it.
func (order StorePluginSort) MetricLabel(key StorePluginSortKey) string {
	switch order {
	case StorePluginSortUpdated:
		if updated, err := time.Parse(storePluginUpdatedLayout, key.DateUpdated); err == nil {
			return updated.Format("2006-01-02")
		}
	case StorePluginSortStars:
		if key.StarCount != nil {
			return fmt.Sprintf("★ %d", *key.StarCount)
		}
	case StorePluginSortName:
	default:
		if key.DownloadCount != nil {
			return fmt.Sprintf("↓ %d", *key.DownloadCount)
		}
	}
	return ""
}

// StoreCatalogID normalizes a catalog ID, treating an empty ID as the official store.
func StoreCatalogID(store string) string {
	store = strings.ToLower(strings.TrimSpace(store))
	if store == "" {
		return OfficialPluginStoreID
	}
	return store
}

// StoreCatalogLess places the official store first when comparing normalized catalog IDs.
func StoreCatalogLess(left, right string) bool {
	if left == OfficialPluginStoreID || right == OfficialPluginStoreID {
		return left == OfficialPluginStoreID && right != OfficialPluginStoreID
	}
	return left < right
}
