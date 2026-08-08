package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gen2brain/malgo"

	"wox/util/mainthread"
)

const filePlayerProgressInterval = 33 * time.Millisecond

// FilePlaybackSnapshot is the observable state of one diagnostic audio file.
type FilePlaybackSnapshot struct {
	Path     string
	Playing  bool
	Position time.Duration
	Duration time.Duration
}

// FilePlayer provides retained play, pause, and progress for a PCM WAV file.
type FilePlayer struct {
	path       string
	pcm        []byte
	sampleRate uint32
	channels   uint16
	duration   time.Duration
	onUpdate   func(FilePlaybackSnapshot)

	allocator *malgo.AllocatedContext
	device    *malgo.Device
	mu        sync.Mutex
	started   bool
	closed    bool
	position  atomic.Int64
	playing   atomic.Bool
	revision  atomic.Uint64
}

// NewFilePlayer loads a PCM WAV file and prepares a portable playback device.
func NewFilePlayer(path string, onUpdate func(FilePlaybackSnapshot)) (*FilePlayer, error) {
	pcm, sampleRate, channels, err := readPCM16WAV(path)
	if err != nil {
		return nil, err
	}
	player := &FilePlayer{
		path: path, pcm: pcm, sampleRate: sampleRate, channels: channels, onUpdate: onUpdate,
		duration: time.Duration(len(pcm)) * time.Second / time.Duration(int(sampleRate)*int(channels)*2),
	}
	var initErr error
	mainthread.Call(func() { initErr = player.initDevice() })
	if initErr != nil {
		return nil, initErr
	}
	player.emit()
	return player, nil
}

func (p *FilePlayer) initDevice() error {
	allocator, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return fmt.Errorf("initialize audio playback context: %w", err)
	}
	config := malgo.DefaultDeviceConfig(malgo.Playback)
	config.Playback.Format = malgo.FormatS16
	config.Playback.Channels = uint32(p.channels)
	config.SampleRate = p.sampleRate
	config.PeriodSizeInMilliseconds = 20
	callbacks := malgo.DeviceCallbacks{Data: func(output, _ []byte, _ uint32) {
		for index := range output {
			output[index] = 0
		}
		if !p.playing.Load() {
			return
		}
		position := p.position.Load()
		if position >= int64(len(p.pcm)) {
			p.playing.Store(false)
			return
		}
		written := copy(output, p.pcm[position:])
		position += int64(written)
		p.position.Store(position)
		if position >= int64(len(p.pcm)) {
			p.playing.Store(false)
		}
	}}
	device, err := malgo.InitDevice(allocator.Context, config, callbacks)
	if err != nil {
		_ = allocator.Uninit()
		allocator.Free()
		return fmt.Errorf("initialize audio playback device: %w", err)
	}
	p.allocator = allocator
	p.device = device
	return nil
}

// Play starts or resumes playback from the retained position.
func (p *FilePlayer) Play() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.device == nil {
		return errors.New("audio file player is closed")
	}
	if p.position.Load() >= int64(len(p.pcm)) {
		p.position.Store(0)
	}
	if !p.started {
		var err error
		mainthread.Call(func() { err = p.device.Start() })
		if err != nil {
			return fmt.Errorf("start audio playback: %w", err)
		}
		p.started = true
	}
	p.playing.Store(true)
	revision := p.revision.Add(1)
	p.emit()
	go p.reportProgress(revision)
	return nil
}

// Pause suspends playback without changing the retained position.
func (p *FilePlayer) Pause() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.revision.Add(1)
	p.playing.Store(false)
	if p.closed || p.device == nil || !p.started {
		p.emit()
		return nil
	}
	var err error
	mainthread.Call(func() { err = p.device.Stop() })
	if err != nil {
		return fmt.Errorf("pause audio playback: %w", err)
	}
	p.started = false
	p.emit()
	return nil
}

// Snapshot returns the current playback position without touching the device.
func (p *FilePlayer) Snapshot() FilePlaybackSnapshot {
	position := time.Duration(p.position.Load()) * time.Second / time.Duration(int(p.sampleRate)*int(p.channels)*2)
	if position > p.duration {
		position = p.duration
	}
	return FilePlaybackSnapshot{Path: p.path, Playing: p.playing.Load(), Position: position, Duration: p.duration}
}

func (p *FilePlayer) reportProgress(revision uint64) {
	ticker := time.NewTicker(filePlayerProgressInterval)
	defer ticker.Stop()
	for range ticker.C {
		if p.revision.Load() != revision {
			return
		}
		p.emit()
		if !p.playing.Load() {
			_ = p.Pause()
			return
		}
	}
}

func (p *FilePlayer) emit() {
	if p.onUpdate != nil {
		p.onUpdate(p.Snapshot())
	}
}

// Close stops playback and releases the native audio device and context.
func (p *FilePlayer) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true
	p.revision.Add(1)
	p.playing.Store(false)
	mainthread.Call(func() {
		if p.device != nil {
			if p.started {
				_ = p.device.Stop()
			}
			p.device.Uninit()
			p.device = nil
		}
		if p.allocator != nil {
			_ = p.allocator.Uninit()
			p.allocator.Free()
			p.allocator = nil
		}
	})
}

func readPCM16WAV(path string) ([]byte, uint32, uint16, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("read audio file: %w", err)
	}
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, 0, 0, errors.New("audio file is not a RIFF/WAVE file")
	}
	var sampleRate uint32
	var channels uint16
	var pcm []byte
	for offset := 12; offset+8 <= len(data); {
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		chunkStart := offset + 8
		chunkEnd := chunkStart + chunkSize
		if chunkEnd > len(data) {
			return nil, 0, 0, errors.New("audio file contains a truncated WAV chunk")
		}
		switch string(data[offset : offset+4]) {
		case "fmt ":
			if chunkSize < 16 || binary.LittleEndian.Uint16(data[chunkStart:chunkStart+2]) != 1 || binary.LittleEndian.Uint16(data[chunkStart+14:chunkStart+16]) != 16 {
				return nil, 0, 0, errors.New("audio file must use 16-bit PCM WAV encoding")
			}
			channels = binary.LittleEndian.Uint16(data[chunkStart+2 : chunkStart+4])
			sampleRate = binary.LittleEndian.Uint32(data[chunkStart+4 : chunkStart+8])
		case "data":
			pcm = append([]byte(nil), data[chunkStart:chunkEnd]...)
		}
		offset = chunkEnd + chunkSize%2
	}
	if sampleRate == 0 || channels == 0 || len(pcm) == 0 {
		return nil, 0, 0, errors.New("audio file is missing PCM format or sample data")
	}
	return pcm, sampleRate, channels, nil
}
