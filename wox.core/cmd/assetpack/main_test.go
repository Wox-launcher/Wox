package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestCompressionDeterministic preserves binary/signature bytes and reproducible output.
func TestCompressionDeterministic(t *testing.T) {
	for _, data := range [][]byte{nil, bytes.Repeat([]byte("text\x00\xff"), 1024), {0, 255, 17}} {
		a, err := compress(data)
		if err != nil {
			t.Fatal(err)
		}
		b, err := compress(data)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Fatal("non-deterministic gzip output")
		}
	}
}

// TestRejectUnwrappedEmbed makes future assets opt into transparent decompression.
func TestRejectUnwrappedEmbed(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		source string
		valid  bool
	}{
		{"package p\nimport \"embed\"\n//go:embed data\nvar raw embed.FS", false},
		{"package p\nimport (\"embed\";\"wox/internal/assetfs\")\n//go:embed data\nvar raw embed.FS\nvar data=assetfs.New(raw)", true},
	} {
		if err := os.WriteFile(filepath.Join(root, "assets.go"), []byte(tc.source), 0644); err != nil {
			t.Fatal(err)
		}
		err := validateConsumers(listedPackage{Dir: root, GoFiles: []string{"assets.go"}})
		if (err == nil) != tc.valid {
			t.Fatalf("valid=%v, err=%v", tc.valid, err)
		}
	}
}
