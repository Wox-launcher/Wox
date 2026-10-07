package assetfs

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"testing"
	"testing/fstest"
)

// testFS exercises both storage modes with identical logical file contents.
func testFS(t *testing.T, compressed bool) FS {
	t.Helper()
	source := fstest.MapFS{}
	for name, data := range map[string][]byte{"empty": {}, "nested/data.bin": {0, 1, 2, 255}, "text": bytes.Repeat([]byte("resource content\n"), 100)} {
		if compressed {
			var b bytes.Buffer
			w := gzip.NewWriter(&b)
			if _, err := w.Write(data); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			data = b.Bytes()
		}
		source[name] = &fstest.MapFile{Data: data, Mode: 0444}
	}
	return FS{source: source, compressed: compressed}
}

// TestFilesystemContract covers directory traversal, file metadata, missing paths,
// read isolation, and the standard io/fs contract in both storage modes.
func TestFilesystemContract(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		f := testFS(t, compressed)
		if err := fstest.TestFS(f, "empty", "nested/data.bin", "text"); err != nil {
			t.Fatal(err)
		}
		got, err := f.ReadFile("nested/data.bin")
		if err != nil || !bytes.Equal(got, []byte{0, 1, 2, 255}) {
			t.Fatalf("read=%v %v", got, err)
		}
		if compressed {
			got[0] = 9
			again, _ := f.ReadFile("nested/data.bin")
			if again[0] != 0 {
				t.Fatal("read mutated shared resource")
			}
		}
		for _, name := range []string{"missing", "../text", "/text"} {
			if _, err := f.ReadFile(name); err == nil {
				t.Fatalf("accepted %q", name)
			}
		}
		entries, err := f.ReadDir("nested")
		if err != nil {
			t.Fatal(err)
		}
		info, err := entries[0].Info()
		if err != nil || info.Size() != 4 {
			t.Fatalf("info=%v err=%v", info, err)
		}
		file, err := f.Open("nested/data.bin")
		if err != nil {
			t.Fatal(err)
		}
		opened, err := io.ReadAll(file)
		file.Close()
		if err != nil || !bytes.Equal(opened, []byte{0, 1, 2, 255}) {
			t.Fatal("Open changed data")
		}
		if _, err := f.Open("missing"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("missing=%v", err)
		}
	}
}

// TestCorruptAssetReturnsError ensures gzip checksum failures never expose partial bytes.
func TestCorruptAssetReturnsError(t *testing.T) {
	f := testFS(t, true)
	source := f.source.(fstest.MapFS)
	source["text"].Data[len(source["text"].Data)-8] ^= 1
	if data, err := f.ReadFile("text"); err == nil || data != nil {
		t.Fatalf("corrupt read=%v, %v", data, err)
	}
}
