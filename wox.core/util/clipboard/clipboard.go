package clipboard

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"wox/util"
)

var noDataErr = errors.New("no such data")
var notImplement = errors.New("not implemented")
var watchMutex sync.Mutex
var watchList = make(map[uint64]*watchSubscription)
var nextWatchID uint64
var isWatching bool
var WatchIntervalMillisecond = 250
var nativeImageFileWriterMu sync.RWMutex
var nativeImageFileWriter func(context.Context, string) error

// selfWriteWindow is how long the watcher stays away from the clipboard around a
// Wox write. It only has to cover the gap between marking the write and claiming
// the change it produces, so ownership can settle without the watcher racing it.
const selfWriteWindow = 200 * time.Millisecond

// lastWriteTimestamp tracks the last time Wox wrote to the clipboard (UnixMilli).
// Used to keep the polling loop away from the clipboard while Wox owns the write.
var lastWriteTimestamp atomic.Int64

// detectClipboardChange reports whether the clipboard changed and consumes the edge
// it reports, so only the first caller after a change ever sees it. It is a variable
// so tests can exercise that contract without a real clipboard.
var detectClipboardChange = isClipboardChanged

// beginSelfWrite opens the window before the platform write so a watcher tick that
// overlaps the write cannot mistake it for somebody else's copy.
func beginSelfWrite() {
	lastWriteTimestamp.Store(time.Now().UnixMilli())
}

// endSelfWrite claims the change edge Wox's own write just produced. The platform
// detectors consume the edge they report, so whoever asks first is the only one who
// sees it. Claiming it at the source is what lets the watcher trust that any edge
// still pending belongs to another application, instead of inferring ownership from
// a timestamp and discarding a user copy that happened to land in the same window.
func endSelfWrite() {
	detectClipboardChange()
	lastWriteTimestamp.Store(time.Now().UnixMilli())
}

// withinSelfWriteWindow reports whether Wox is still settling its own write.
func withinSelfWriteWindow() bool {
	return time.Since(time.UnixMilli(lastWriteTimestamp.Load())) < selfWriteWindow
}

// claimExternalChange reports whether this tick owns a change made outside Wox.
// The order is the whole point: the platform detectors consume the edge they
// report, so asking first and bailing afterwards destroys the only notice of an
// external copy that landed next to a Wox write. Skipping first leaves that edge
// pending for a later tick, and Wox's own edge is already claimed by endSelfWrite.
func claimExternalChange() bool {
	if withinSelfWriteWindow() {
		util.GetLogger().Info(context.Background(), "clipboard: watcher tick skipped while Wox owns the write")
		return false
	}
	return detectClipboardChange()
}

// NoDataErr returns the sentinel error reported when the clipboard contains no recognizable data.
func NoDataErr() error { return noDataErr }

// SetNativeImageFileWriter registers a UI-owned image clipboard writer for platforms where
// background clipboard ownership is restricted by the compositor.
func SetNativeImageFileWriter(writer func(context.Context, string) error) {
	nativeImageFileWriterMu.Lock()
	defer nativeImageFileWriterMu.Unlock()
	nativeImageFileWriter = writer
}

// writeNativeImageFile calls the registered UI-owned image clipboard writer.
func writeNativeImageFile(ctx context.Context, filePath string) error {
	nativeImageFileWriterMu.RLock()
	writer := nativeImageFileWriter
	nativeImageFileWriterMu.RUnlock()
	if writer == nil {
		return errors.New("clipboard: native image file writer is not registered")
	}
	return writer(ctx, filePath)
}

type Type string

const (
	ClipboardTypeText  Type = "text"
	ClipboardTypeImage Type = "image"
	ClipboardTypeFile  Type = "file"
)

type Data interface {
	GetType() Type
	String() string
	MarshalJSON() ([]byte, error)
	UnmarshalJSON([]byte) error
}

type watchSubscription struct {
	mutex    sync.Mutex
	callback func(Data)
	closed   bool
}

// call serializes one subscriber so unsubscribe can wait for an active callback.
func (s *watchSubscription) call(data Data) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if !s.closed {
		s.callback(data)
	}
}

// close prevents future callbacks after the current callback returns.
func (s *watchSubscription) close() {
	s.mutex.Lock()
	s.closed = true
	s.mutex.Unlock()
}

func Read() (Data, error) {
	contentType := readClipboardContentType()
	switch contentType {
	case ClipboardTypeText:
		text, err := readText()
		if err != nil {
			return nil, err
		}
		return &TextData{Text: text}, nil
	case ClipboardTypeImage:
		img, err := readImage()
		if err != nil {
			return nil, err
		}
		return &ImageData{Image: img}, nil
	case ClipboardTypeFile:
		paths, err := readFilePaths()
		if err != nil {
			return nil, err
		}
		return &FilePathData{FilePaths: paths}, nil
	default:
		return nil, noDataErr
	}
}

func ReadFilesAndText() (Data, error) {
	filePaths, fileErr := readFilePaths()
	if fileErr == nil {
		return &FilePathData{
			FilePaths: filePaths,
		}, nil
	}

	textData, txtErr := readText()
	if txtErr == nil {
		return &TextData{
			Text: textData,
		}, nil
	}

	return nil, noDataErr
}

func Write(data Data) error {
	beginSelfWrite()
	switch data.GetType() {
	case ClipboardTypeText:
		if err := writeTextData(data.String()); err != nil {
			util.GetLogger().Error(context.Background(), fmt.Sprintf("clipboard: write text failed: %v", err))
			return err
		}
	case ClipboardTypeImage:
		if err := writeImageData(data.(*ImageData).Image); err != nil {
			util.GetLogger().Error(context.Background(), fmt.Sprintf("clipboard: write image failed: %v", err))
			return err
		}
	case ClipboardTypeFile:
		if err := writeFilePaths(data.(*FilePathData).FilePaths); err != nil {
			util.GetLogger().Error(context.Background(), fmt.Sprintf("clipboard: write file paths failed: %v", err))
			return err
		}
	default:
		return errors.New("not implemented")
	}

	// A failed write leaves the clipboard untouched, so only a write that landed
	// may claim a change edge; claiming one otherwise would eat somebody else's.
	endSelfWrite()
	return nil
}

func WriteImageBytes(pngData []byte, dibData []byte) error {
	beginSelfWrite()
	if err := writeImageBytes(pngData, dibData); err != nil {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("clipboard: write image bytes failed: %v", err))
		return err
	}
	endSelfWrite()
	return nil
}

// Watch subscribes to clipboard changes and returns a function that waits for any active callback before unsubscribing.
func Watch(cb func(Data)) func() {
	watchMutex.Lock()
	nextWatchID++
	id := nextWatchID
	subscription := &watchSubscription{callback: cb}
	watchList[id] = subscription
	if !isWatching {
		isWatching = true
		go func() {
			for {
				time.Sleep(time.Millisecond * time.Duration(WatchIntervalMillisecond))
				watchChange()
			}
		}()
	}
	watchMutex.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			watchMutex.Lock()
			delete(watchList, id)
			watchMutex.Unlock()
			subscription.close()
		})
	}
}

func watchChange() {
	defer func() {
		if err := recover(); err != nil {
			util.GetLogger().Error(context.Background(), fmt.Sprintf("clipboard: watchChange panic: %v", err))
		}
	}()

	if !claimExternalChange() {
		return
	}

	// Debounce: wait briefly to let the clipboard settle.
	// When the user rapidly copies items, this avoids opening the clipboard
	// while the source application is still writing, reducing lock contention.
	settleStart := time.Now()
	settleChanges := 0
	for {
		time.Sleep(50 * time.Millisecond)
		// If the clipboard changed during the sleep, it means the user is still copying or the source app is still writing.
		// We loop until it settles (no sequence number change for 50ms).
		if !detectClipboardChange() {
			break
		}
		settleChanges++

		// Unblock after maximum 1 second to prevent permanent deadlocks
		// from badly behaving apps that continuously update the clipboard.
		if time.Since(settleStart) > time.Second {
			util.GetLogger().Warn(context.Background(), "clipboard: debounce timeout exceeded, reading anyway")
			break
		}
	}
	if settleChanges > 0 {
		util.GetLogger().Warn(
			context.Background(),
			fmt.Sprintf("clipboard: debounce observed %d additional changes before read (elapsed=%s)", settleChanges, time.Since(settleStart).String()),
		)
	}

	start := time.Now()
	data, err := Read()
	if err != nil {
		snapshot := buildWatchSnapshot()
		if snapshot != "" {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("clipboard: changed but failed to read: %v %s", err, snapshot))
		} else {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("clipboard: changed but failed to read: %v", err))
		}
		return
	}

	if d := time.Since(start); d > 200*time.Millisecond {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("clipboard: Read took %s (type=%s)", d.String(), data.GetType()))
	}

	watchMutex.Lock()
	subscriptions := make([]*watchSubscription, 0, len(watchList))
	for _, subscription := range watchList {
		subscriptions = append(subscriptions, subscription)
	}
	watchMutex.Unlock()

	for _, subscription := range subscriptions {
		go func(subscription *watchSubscription) {
			defer func() {
				if err1 := recover(); err1 != nil {
					util.GetLogger().Error(context.Background(), fmt.Sprintf("clipboard: callback panic: %v", err1))
				}
			}()

			subscription.call(data)
		}(subscription)
	}
}

func WriteText(text string) error {
	return Write(&TextData{
		Text: text,
	})
}

// ReadText returns the current clipboard text and a nil error when the clipboard holds text.
// An empty clipboard (no recognizable data) returns "" + nil. Non-text data returns "" + nil
// so callers that only care about text can treat it as "nothing to paste".
func ReadText() (string, error) {
	data, err := Read()
	if err != nil {
		if errors.Is(err, noDataErr) {
			return "", nil
		}
		return "", err
	}
	if text, ok := data.(*TextData); ok {
		return text.Text, nil
	}
	return "", nil
}

type TextData struct {
	Text string
}

func (t *TextData) GetType() Type {
	return ClipboardTypeText
}

func (t *TextData) String() string {
	return t.Text
}

func (t *TextData) MarshalJSON() ([]byte, error) {
	var mapData = make(map[string]string)
	mapData["text"] = t.Text
	mapData["type"] = string(t.GetType())
	return json.Marshal(mapData)
}

func (t *TextData) UnmarshalJSON(data []byte) error {
	var mapData = make(map[string]string)
	err := json.Unmarshal(data, &mapData)
	if err != nil {
		return err
	}

	t.Text = mapData["text"]
	return nil
}

type FilePathData struct {
	FilePaths []string
}

func (f *FilePathData) GetType() Type {
	return ClipboardTypeFile
}

func (f *FilePathData) String() string {
	return strings.Join(f.FilePaths, ";")
}

func (f *FilePathData) MarshalJSON() ([]byte, error) {
	var mapData = make(map[string]string)
	mapData["filePaths"] = strings.Join(f.FilePaths, "``")
	mapData["type"] = string(f.GetType())
	return json.Marshal(mapData)
}

func (f *FilePathData) UnmarshalJSON(data []byte) error {
	var mapData = make(map[string]string)
	err := json.Unmarshal(data, &mapData)
	if err != nil {
		return err
	}

	f.FilePaths = strings.Split(mapData["filePaths"], "``")
	return nil
}

type ImageData struct {
	Image image.Image
}

func (i *ImageData) GetType() Type {
	return ClipboardTypeImage
}

func (i *ImageData) String() string {
	b := i.Image.Bounds()
	return fmt.Sprintf("image(%dx%d)", b.Dx(), b.Dy())
}

func (i *ImageData) MarshalJSON() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := png.Encode(buf, i.Image)
	if err != nil {
		return nil, err
	}

	return json.Marshal(base64.StdEncoding.EncodeToString(buf.Bytes()))
}

func (i *ImageData) UnmarshalJSON(data []byte) error {
	var base64ImgData string
	unmarshalErr := json.Unmarshal(data, &base64ImgData)
	if unmarshalErr != nil {
		return unmarshalErr
	}

	decodeBytes, err := base64.StdEncoding.DecodeString(base64ImgData)
	if err != nil {
		return err
	}

	img, decodeErr := png.Decode(bytes.NewReader(decodeBytes))
	if decodeErr != nil {
		return decodeErr
	}

	i.Image = img
	return nil
}
