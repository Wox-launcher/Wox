package launcher

import (
	"log"

	previewview "wox/ui/launcher/view/preview"
	"wox/util/audio"
)

type dictationPreviewAudioState struct {
	revision uint64
	path     string
	player   *audio.FilePlayer
	snapshot audio.FilePlaybackSnapshot
}

func dictationPlaybackProps(snapshot audio.FilePlaybackSnapshot) previewview.DictationAudioPlayback {
	return previewview.DictationAudioPlayback{Playing: snapshot.Playing, Position: snapshot.Position, Duration: snapshot.Duration}
}

// reconcileDictationAudioPreview releases a track that does not belong to the selected history result.
func (a *App) reconcileDictationAudioPreview(preview queryPreview) {
	if a.dictationAudio == nil {
		return
	}
	data, err := decodeStructuredPreview[dictationHistoryPreviewData](preview.PreviewData)
	if err != nil || (a.dictationAudio.path != data.RawAudioPath && a.dictationAudio.path != data.ProcessedAudioPath) {
		a.deactivateDictationAudio()
	}
}

// dictationAudioSnapshot returns playback state only for the requested track.
func (a *App) dictationAudioSnapshot(path string) audio.FilePlaybackSnapshot {
	if a.dictationAudio == nil || a.dictationAudio.path != path {
		return audio.FilePlaybackSnapshot{Path: path}
	}
	return a.dictationAudio.snapshot
}

// toggleDictationAudio creates, pauses, or resumes the selected diagnostic track.
func (a *App) toggleDictationAudio(path string) {
	if path == "" {
		return
	}
	if state := a.dictationAudio; state != nil && state.path == path && state.player != nil {
		player := state.player
		playing := state.snapshot.Playing
		go func() {
			var err error
			if playing {
				err = player.Pause()
			} else {
				err = player.Play()
			}
			if err != nil {
				log.Printf("toggle dictation preview audio: %v", err)
			}
		}()
		return
	}

	a.deactivateDictationAudio()
	state := &dictationPreviewAudioState{revision: 1, path: path, snapshot: audio.FilePlaybackSnapshot{Path: path}}
	a.dictationAudio = state
	if a.window != nil {
		_ = a.window.Invalidate()
	}
	go a.loadDictationAudio(state, state.revision)
}

func (a *App) loadDictationAudio(state *dictationPreviewAudioState, revision uint64) {
	player, err := audio.NewFilePlayer(state.path, func(snapshot audio.FilePlaybackSnapshot) {
		_ = a.uiCall(func() {
			if a.dictationAudio != state || state.revision != revision {
				return
			}
			state.snapshot = snapshot
			if a.window != nil {
				_ = a.window.Invalidate()
			}
		})
	})
	if err != nil {
		_ = a.uiCall(func() {
			if a.dictationAudio == state && state.revision == revision {
				log.Printf("load dictation preview audio: %v", err)
				if a.window != nil {
					_ = a.window.Invalidate()
				}
			}
		})
		return
	}
	stale := false
	if err := a.uiCall(func() {
		if a.dictationAudio != state || state.revision != revision {
			stale = true
			return
		}
		state.player = player
		state.snapshot = player.Snapshot()
		go func() {
			if playErr := player.Play(); playErr != nil {
				log.Printf("play dictation preview audio: %v", playErr)
			}
		}()
	}); err != nil {
		player.Close()
		log.Printf("attach dictation preview audio: %v", err)
		return
	}
	if stale {
		player.Close()
	}
}

// deactivateDictationAudio stops playback and releases its device outside rendering.
func (a *App) deactivateDictationAudio() {
	state := a.dictationAudio
	if state == nil {
		return
	}
	state.revision++
	a.dictationAudio = nil
	if state.player != nil {
		player := state.player
		go player.Close()
	}
}
