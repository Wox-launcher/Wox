package filesearch

import "errors"

// ReleaseIdleMemory participates in the launcher's existing long-hide cleanup.
// It preserves the durable overlay and lends no mapped names across the trim.
func (e *Engine) ReleaseIdleMemory() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.closed {
		return nil
	}
	var err error
	if e.db != nil && e.db.directory != nil {
		err = e.db.directory.releaseIdleMemory()
	}
	if e.nativeIndex != nil {
		err = errors.Join(err, e.nativeIndex.directoryView().releaseIdleMemory())
	}
	return err
}

// releaseIdleMemory serializes with shard attachment, queries and shutdown.
func (d *directoryIndex) releaseIdleMemory() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	var err error
	for _, shard := range d.shards {
		err = errors.Join(err, shard.store.ReleaseIdleMemory())
	}
	return err
}
