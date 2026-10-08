//go:build linux

package clipboard

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// installOwnedPortalSelection provides an initialized owner without a D-Bus connection, so local reads must bypass SelectionRead.
func installOwnedPortalSelection(t *testing.T, contentType Type, mimeType string, payload []byte) {
	t.Helper()
	linuxPortalMu.Lock()
	previous := linuxClipboardPortal
	linuxClipboardPortal = linuxPortalClipboard{
		sessionPath:   "/test/clipboard",
		signals:       make(chan *dbus.Signal, 4),
		latest:        portalClipboardOffer{contentType: contentType, mimeTypes: []string{mimeType}},
		writePayloads: map[string][]byte{mimeType: payload},
		ownsSelection: true,
		initialized:   true,
	}
	linuxPortalMu.Unlock()
	t.Cleanup(func() {
		linuxPortalMu.Lock()
		linuxClipboardPortal = previous
		linuxPortalMu.Unlock()
	})
}

// TestPortalOwnTextReachesWatcher covers the full claim and read path after a Wox copy settles.
func TestPortalOwnTextReachesWatcher(t *testing.T) {
	installOwnedPortalSelection(t, ClipboardTypeText, portalMimeTextUTF8, []byte("Wox copy"))
	installFakeClipboardEdge(t)
	detectClipboardChange = portalIsChanged
	// A pre-write notification can be queued ahead of the acknowledgement of our copy.
	linuxClipboardPortal.signals <- &dbus.Signal{
		Name: portalSelectionOwnerChangedSignal,
		Body: []interface{}{linuxClipboardPortal.sessionPath, map[string]dbus.Variant{
			"session_is_owner": dbus.MakeVariant(false),
			"mime_types":       dbus.MakeVariant([]string{}),
		}},
	}
	linuxClipboardPortal.signals <- &dbus.Signal{
		Name: portalSelectionOwnerChangedSignal,
		Body: []interface{}{linuxClipboardPortal.sessionPath, map[string]dbus.Variant{
			"session_is_owner": dbus.MakeVariant(true),
			"mime_types":       dbus.MakeVariant([]string{portalMimeTextUTF8}),
		}},
	}
	beginSelfWrite()
	endSelfWrite()
	lastWriteTimestamp.Store(time.Now().Add(-2 * selfWriteWindow).UnixMilli())
	if !claimClipboardChange() {
		t.Fatal("watcher dropped the portal's own copy")
	}
	if got := portalReadContentType(); got != ClipboardTypeText {
		t.Fatalf("content type = %q, want text", got)
	}
	text, err := portalReadText()
	if err != nil || text != "Wox copy" {
		t.Fatalf("read text = %q, %v", text, err)
	}
	if claimClipboardChange() {
		t.Fatal("portal's own copy was claimed twice")
	}
}

// TestPortalReadsOwnFiles checks that owned URI-list bytes pass through normal file decoding.
func TestPortalReadsOwnFiles(t *testing.T) {
	installOwnedPortalSelection(t, ClipboardTypeFile, portalMimeURIList, []byte("file:///tmp/a%20b.txt\r\nfile:///tmp/c.txt\r\n"))
	paths, err := portalReadFilePaths()
	if err != nil || !reflect.DeepEqual(paths, []string{"/tmp/a b.txt", "/tmp/c.txt"}) {
		t.Fatalf("read files = %v, %v", paths, err)
	}
}

// TestPortalOwnImageSnapshotKeepsItsBytes checks that deferred decoding survives a later provider payload change.
func TestPortalOwnImageSnapshotKeepsItsBytes(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 3))
	img.SetNRGBA(0, 0, color.NRGBA{R: 200, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	installOwnedPortalSelection(t, ClipboardTypeImage, portalMimePNG, encoded.Bytes())
	snapshot, err := portalReadImageSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	clear(linuxClipboardPortal.writePayloads[portalMimePNG])
	decoded, err := snapshot.Decode(0)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != img.Bounds() || decoded.At(0, 0) != img.At(0, 0) {
		t.Fatal("snapshot lost the original pixels")
	}
}

// TestPortalOwnerChangeDisablesLocalReads covers equal-format, empty, and unrelated-session notifications.
func TestPortalOwnerChangeDisablesLocalReads(t *testing.T) {
	for _, test := range []struct {
		name      string
		session   dbus.ObjectPath
		mimeTypes []string
		wantOwn   bool
		wantType  Type
	}{
		{"same offer", "/test/clipboard", []string{portalMimePNG}, false, ClipboardTypeImage},
		{"empty selection", "/test/clipboard", nil, false, ""},
		{"another session", "/test/other", nil, true, ClipboardTypeImage},
	} {
		t.Run(test.name, func(t *testing.T) {
			installOwnedPortalSelection(t, ClipboardTypeImage, portalMimePNG, []byte("owned payload"))
			linuxClipboardPortal.latest.fingerprint = portalMimePNG
			signal := &dbus.Signal{Body: []interface{}{test.session, map[string]dbus.Variant{
				"session_is_owner": dbus.MakeVariant(false),
				"mime_types":       dbus.MakeVariant(test.mimeTypes),
			}}}
			linuxPortalMu.Lock()
			handleLinuxPortalSelectionOwnerChangedLocked(signal)
			linuxPortalMu.Unlock()
			if linuxClipboardPortal.ownsSelection != test.wantOwn {
				t.Fatalf("owns selection = %v, want %v", linuxClipboardPortal.ownsSelection, test.wantOwn)
			}
			if got := portalReadContentType(); got != test.wantType {
				t.Fatalf("content type = %q, want %q", got, test.wantType)
			}
		})
	}
}

// TestPortalOwnedPayloadIsImmutable checks that callers cannot mutate provider storage or read unadvertised formats.
func TestPortalOwnedPayloadIsImmutable(t *testing.T) {
	installOwnedPortalSelection(t, ClipboardTypeText, portalMimeTextUTF8, []byte("Wox copy"))
	linuxPortalMu.Lock()
	defer linuxPortalMu.Unlock()
	data, err := readLinuxPortalSelectionLocked(portalMimeTextUTF8)
	if err != nil {
		t.Fatal(err)
	}
	data[0] = 'X'
	if got := string(linuxClipboardPortal.writePayloads[portalMimeTextUTF8]); got != "Wox copy" {
		t.Fatalf("provider payload changed to %q", got)
	}
	if _, err := readLinuxPortalSelectionLocked(portalMimePNG); !errors.Is(err, noDataErr) {
		t.Fatalf("missing format error = %v, want noDataErr", err)
	}
}
