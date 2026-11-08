package plugin

import "wox/util"

// pluginPreviewRegistry holds installed callbacks independently of query-result eviction.
type pluginPreviewRegistry struct {
	sessions *util.HashMap[string, *pluginPreviewBucket]
}

type pluginPreviewKey struct{ query, owner, document string }
type pluginPreviewBucket struct {
	active  *pluginPreviewEntry
	retired map[pluginPreviewKey]*pluginPreviewEntry
}

type pluginPreviewEntry struct {
	owner   *Instance
	queryID string
	preview WoxPreview
}

func newPluginPreviewRegistry() *pluginPreviewRegistry {
	return &pluginPreviewRegistry{sessions: util.NewHashMap[string, *pluginPreviewBucket]()}
}

// install keeps the first preview in one plugin context and rejects duplicate candidates.
func (r *pluginPreviewRegistry) install(session, query string, owner *Instance, preview WoxPreview) bool {
	if session == "" || query == "" || owner == nil || owner.Metadata.Id == "" || preview.WoxViewID == "" || preview.PreviewType != WoxPreviewTypeWoxView {
		return false
	}
	accepted := false
	r.sessions.Compute(session, func(bucket *pluginPreviewBucket, exists bool) (*pluginPreviewBucket, bool) {
		if !exists {
			bucket = &pluginPreviewBucket{retired: make(map[pluginPreviewKey]*pluginPreviewEntry)}
		}
		if bucket.active != nil {
			return bucket, true
		}
		accepted = true
		bucket.active = &pluginPreviewEntry{owner: owner, queryID: query, preview: preview}
		return bucket, true
	})
	return accepted
}

// find validates the originating session and document before dispatching a callback.
func (r *pluginPreviewRegistry) find(session, query, owner, document string) (*pluginPreviewEntry, bool) {
	var entry *pluginPreviewEntry
	r.sessions.Compute(session, func(bucket *pluginPreviewBucket, exists bool) (*pluginPreviewBucket, bool) {
		if !exists || document == "" {
			return bucket, exists
		}
		key := pluginPreviewKey{query, owner, document}
		if current := bucket.active; current != nil && current.queryID == query && current.owner.Metadata.Id == owner && current.preview.WoxViewID == document {
			entry = current
		} else {
			entry = bucket.retired[key]
		}
		return bucket, true
	})
	return entry, entry != nil
}

// retire detaches the current document immediately while preserving its queued hide callback.
func (r *pluginPreviewRegistry) retire(session, query, owner, document string) bool {
	retired := false
	r.sessions.Compute(session, func(bucket *pluginPreviewBucket, exists bool) (*pluginPreviewBucket, bool) {
		if !exists || bucket.active == nil || document == "" {
			return bucket, exists
		}
		entry := bucket.active
		if entry.queryID == query && entry.owner.Metadata.Id == owner && entry.preview.WoxViewID == document {
			bucket.retired[pluginPreviewKey{query, owner, document}] = entry
			bucket.active = nil
			retired = true
		}
		return bucket, true
	})
	return retired
}

// release only removes the expected document, so a late hide cannot revoke its replacement.
func (r *pluginPreviewRegistry) release(session, query, owner, document string) bool {
	removed := false
	r.sessions.Compute(session, func(bucket *pluginPreviewBucket, ok bool) (*pluginPreviewBucket, bool) {
		if !ok || document == "" {
			return bucket, ok
		}
		key := pluginPreviewKey{query, owner, document}
		if entry := bucket.active; entry != nil && entry.queryID == query && entry.owner.Metadata.Id == owner && entry.preview.WoxViewID == document {
			bucket.active = nil
			removed = true
		} else if _, found := bucket.retired[key]; found {
			delete(bucket.retired, key)
			removed = true
		}
		return bucket, bucket.active != nil || len(bucket.retired) > 0
	})
	return removed
}
