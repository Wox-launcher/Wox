package util

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyPathRecursiveSkipAndOnError(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "copied")

	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "skip.txt"), []byte("skip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "nested.txt"), []byte("nested"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := CopyPath(src, dst, CopyOptions{
		SkipSymlinks: true,
		Skip: func(info os.FileInfo, _, _ string) (bool, error) {
			return info.Name() == "skip.txt", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got, err := os.ReadFile(filepath.Join(dst, "keep.txt")); err != nil || string(got) != "keep" {
		t.Fatalf("keep.txt = %q, %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(dst, "sub", "nested.txt")); err != nil || string(got) != "nested" {
		t.Fatalf("sub/nested.txt = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "skip.txt")); !os.IsNotExist(err) {
		t.Fatalf("skip.txt should not have been copied: %v", err)
	}

	handled := false
	sentinel := errors.New("boom")
	err = CopyPath(src, filepath.Join(t.TempDir(), "err"), CopyOptions{
		Skip: func(os.FileInfo, string, string) (bool, error) {
			return false, sentinel
		},
		OnError: func(_, _ string, err error) error {
			handled = errors.Is(err, sentinel)
			return nil
		},
	})
	if err != nil || !handled {
		t.Fatalf("OnError did not intercept error: err=%v handled=%v", err, handled)
	}
}

func TestExpandHomeDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "", want: ""},
		{in: "/abs/path", want: "/abs/path"},
		{in: "~", want: home},
		{in: "~/docs/file.txt", want: filepath.Join(home, "docs", "file.txt")},
		{in: `~\docs`, want: filepath.Join(home, "docs")},
		{in: "~other/docs", wantErr: true},
	}

	for _, tc := range cases {
		got, err := ExpandHomeDir(tc.in)
		if (err != nil) != tc.wantErr {
			t.Fatalf("ExpandHomeDir(%q) err=%v, wantErr=%v", tc.in, err, tc.wantErr)
		}
		if !tc.wantErr && got != tc.want {
			t.Fatalf("ExpandHomeDir(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
