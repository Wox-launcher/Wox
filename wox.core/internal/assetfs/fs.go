// Package assetfs provides transparent per-file compression for Wox-owned assets.
package assetfs

import (
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
)

// FS preserves embedded paths and metadata without retaining decompressed files.
// Each read owns its bytes, so concurrent consumers cannot mutate shared assets.
type FS struct {
	source     fs.ReadFileFS
	compressed bool
}

func New(source embed.FS) FS { return FS{source: source, compressed: packed} }

// ReadFile restores the exact original bytes, including executable signatures.
func (f FS) ReadFile(name string) ([]byte, error) {
	data, err := f.source.ReadFile(name)
	if err != nil || !f.compressed {
		return data, err
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, &fs.PathError{Op: "read", Path: name, Err: err}
	}
	defer reader.Close()
	data, err = io.ReadAll(reader)
	if err != nil {
		return nil, &fs.PathError{Op: "read", Path: name, Err: err}
	}
	return data, nil
}

// Open supports the standard io/fs helpers as well as streaming file consumers.
func (f FS) Open(name string) (fs.File, error) {
	if !f.compressed {
		return f.source.Open(name)
	}
	raw, err := f.source.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := raw.Stat()
	if err != nil {
		raw.Close()
		return nil, err
	}
	if info.IsDir() {
		return &directory{ReadDirFile: raw.(fs.ReadDirFile), owner: f, name: name}, nil
	}
	raw.Close()
	data, err := f.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return &file{Reader: bytes.NewReader(data), info: fileInfo{FileInfo: info, size: int64(len(data))}}, nil
}

// ReadDir keeps the original sorted names and reports uncompressed file sizes.
func (f FS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(f.source, name)
	if err != nil || !f.compressed {
		return entries, err
	}
	return f.wrapEntries(name, entries), nil
}

// MustReadFile is reserved for mandatory built-in assets whose public accessors
// cannot return an error. Corrupt release assets must fail instead of returning bad bytes.
func (f FS) MustReadFile(name string) []byte {
	data, err := f.ReadFile(name)
	if err != nil {
		panic(fmt.Sprintf("invalid embedded asset %s: %v", name, err))
	}
	return data
}

// wrapEntries defers size lookup until callers request metadata.
func (f FS) wrapEntries(parent string, entries []fs.DirEntry) []fs.DirEntry {
	for i, entry := range entries {
		name := entry.Name()
		if parent != "." {
			name = parent + "/" + name
		}
		entries[i] = dirEntry{DirEntry: entry, owner: f, name: name}
	}
	return entries
}

type dirEntry struct {
	fs.DirEntry
	owner FS
	name  string
}

// Info reads only compressed metadata; directory walks never inflate asset data.
func (e dirEntry) Info() (fs.FileInfo, error) {
	info, err := e.DirEntry.Info()
	if err != nil || info.IsDir() {
		return info, err
	}
	data, err := e.owner.source.ReadFile(e.name)
	if err != nil {
		return nil, err
	}
	// The packer rejects files >= 4 GiB, so gzip's ISIZE is the exact original size.
	if len(data) < 18 {
		return nil, &fs.PathError{Op: "stat", Path: e.name, Err: fs.ErrInvalid}
	}
	return fileInfo{FileInfo: info, size: int64(binary.LittleEndian.Uint32(data[len(data)-4:]))}, nil
}

type fileInfo struct {
	fs.FileInfo
	size int64
}

func (f fileInfo) Size() int64 { return f.size }

type directory struct {
	fs.ReadDirFile
	owner FS
	name  string
}

func (d *directory) ReadDir(n int) ([]fs.DirEntry, error) {
	entries, err := d.ReadDirFile.ReadDir(n)
	return d.owner.wrapEntries(d.name, entries), err
}

type file struct {
	*bytes.Reader
	info   fs.FileInfo
	closed bool
}

func (f *file) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *file) Close() error               { f.closed = true; return nil }
func (f *file) Read(p []byte) (int, error) {
	if f.closed {
		return 0, fs.ErrClosed
	}
	return f.Reader.Read(p)
}
