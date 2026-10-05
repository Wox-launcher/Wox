// Package screenshotedit owns durable screenshot scenes and their clipboard lookup index.
package screenshotedit

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sync"

	"wox/util/clipboard"
)

const Suffix = ".edit.zip"

// Metadata contains only portable scene data; image resources stay in separate ZIP entries.
type Metadata struct {
	Version     int
	ImageHashes []string
	ExportHash  string
	State       json.RawMessage
}

var index = struct {
	sync.RWMutex
	hashes map[string]map[string]bool
}{hashes: make(map[string]map[string]bool)}

// Save commits a self-contained scene before publishing its clipboard fingerprints.
func Save(path string, state json.RawMessage, source, cursor, composited, window, background image.Image) error {
	exportHash, err := fileHash(path)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	exported, _, err := image.Decode(file)
	_ = file.Close()
	if err != nil {
		return err
	}
	metadata := Metadata{Version: 1, ExportHash: exportHash, State: state,
		ImageHashes: []string{clipboard.ImageHash(composited), clipboard.ImageHash(exported)}}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".screenshot-edit-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	archive := zip.NewWriter(tmp)
	entry, err := archive.Create("scene.json")
	if err != nil {
		return err
	}
	if err = json.NewEncoder(entry).Encode(metadata); err != nil {
		return err
	}
	for _, resource := range []struct {
		name string
		img  image.Image
	}{{"source.png", source}, {"cursor.png", cursor}, {"window.png", window}, {"background.png", background}} {
		if resource.img == nil {
			continue
		}
		// PNG is already compressed; deflating it again only delays screenshot completion.
		entry, err = archive.CreateHeader(&zip.FileHeader{Name: resource.name, Method: zip.Store})
		if err != nil {
			return err
		}
		if err = png.Encode(entry, resource.img); err != nil {
			return err
		}
	}
	if err = archive.Close(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	// Retention can delete the exported image while the full desktop PNG is encoding.
	// Serialize commit with scene removal and never publish data for a missing/replaced image.
	index.Lock()
	defer index.Unlock()
	currentHash, err := fileHash(path)
	if err != nil {
		return err
	}
	if currentHash != exportHash {
		return errors.New("screenshot changed while saving its scene")
	}
	if err = os.Rename(tmp.Name(), path+Suffix); err != nil {
		return err
	}
	registerIndexPath(path, metadata)
	return nil
}

// ReadMetadata avoids decoding desktop images while rebuilding the clipboard index.
func ReadMetadata(path string) (Metadata, error) {
	archive, err := zip.OpenReader(path + Suffix)
	if err != nil {
		return Metadata{}, err
	}
	defer archive.Close()
	return readMetadata(archive.File)
}

// readMetadata bounds JSON allocation and rejects unsupported scene versions.
func readMetadata(files []*zip.File) (Metadata, error) {
	for _, file := range files {
		if file.Name != "scene.json" {
			continue
		}
		if file.UncompressedSize64 > 16<<20 {
			return Metadata{}, errors.New("screenshot scene is too large")
		}
		reader, err := file.Open()
		if err != nil {
			return Metadata{}, err
		}
		defer reader.Close()
		var metadata Metadata
		err = json.NewDecoder(io.LimitReader(reader, 16<<20)).Decode(&metadata)
		if err != nil {
			return Metadata{}, err
		}
		if metadata.Version != 1 || len(metadata.State) == 0 || len(metadata.ExportHash) != 64 || len(metadata.ImageHashes) != 2 {
			return Metadata{}, errors.New("unsupported or invalid screenshot scene")
		}
		for _, hash := range append([]string{metadata.ExportHash}, metadata.ImageHashes...) {
			if decoded, err := hex.DecodeString(hash); err != nil || len(decoded) != sha256.Size {
				return Metadata{}, errors.New("invalid screenshot fingerprint")
			}
		}
		return metadata, nil
	}
	return Metadata{}, errors.New("screenshot scene is missing")
}

// Load verifies the exported file before allocating the scene's lossless image resources.
func Load(path string) (Metadata, image.Image, image.Image, image.Image, image.Image, error) {
	archive, err := zip.OpenReader(path + Suffix)
	if err != nil {
		return Metadata{}, nil, nil, nil, nil, err
	}
	defer archive.Close()
	metadata, err := readMetadata(archive.File)
	if err != nil {
		return Metadata{}, nil, nil, nil, nil, err
	}
	hash, err := fileHash(path)
	if err != nil {
		return Metadata{}, nil, nil, nil, nil, err
	}
	if hash != metadata.ExportHash {
		return Metadata{}, nil, nil, nil, nil, errors.New("screenshot image no longer matches its scene")
	}
	var source, cursor, window, background image.Image
	for _, file := range archive.File {
		if file.Name != "source.png" && file.Name != "cursor.png" && file.Name != "window.png" && file.Name != "background.png" {
			continue
		}
		img, err := readImage(file)
		if err != nil {
			return Metadata{}, nil, nil, nil, nil, err
		}
		if file.Name == "source.png" {
			source = img
		} else if file.Name == "cursor.png" {
			cursor = img
		} else if file.Name == "window.png" {
			window = img
		} else {
			background = img
		}
	}
	if source == nil {
		return Metadata{}, nil, nil, nil, nil, errors.New("screenshot source image is missing")
	}
	return metadata, source, cursor, window, background, nil
}

// readImage checks decoded dimensions before a corrupt local archive can allocate arbitrary memory.
func readImage(file *zip.File) (image.Image, error) {
	if file.UncompressedSize64 > 1<<30 {
		return nil, errors.New("screenshot image is too large")
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	config, err := png.DecodeConfig(reader)
	_ = reader.Close()
	limit := int64(256 * 1024 * 1024)
	if file.Name == "cursor.png" {
		limit = 1024 * 1024
	}
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width) > limit/int64(config.Height) {
		return nil, errors.New("invalid screenshot image dimensions")
	}
	reader, err = file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return png.Decode(reader)
}

// fileHash detects replacement of an exported screenshot without trusting timestamps.
func fileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Available checks ownership files without decoding image or scene contents during a query.
func Available(path string) bool {
	imageInfo, err := os.Stat(path)
	if err != nil || imageInfo.IsDir() {
		return false
	}
	sceneInfo, err := os.Stat(path + Suffix)
	return err == nil && !sceneInfo.IsDir()
}

// Register replaces one path's fingerprints, so startup and new captures can safely overlap.
func Register(path string, metadata Metadata) {
	index.Lock()
	defer index.Unlock()
	registerIndexPath(path, metadata)
}

// registerIndexPath publishes both fingerprints while the archive commit holds the index lock.
func registerIndexPath(path string, metadata Metadata) {
	removeIndexPath(path)
	for _, hash := range metadata.ImageHashes {
		if hash == "" {
			continue
		}
		if index.hashes[hash] == nil {
			index.hashes[hash] = make(map[string]bool)
		}
		index.hashes[hash][path] = true
	}
}

// Resolve refuses ambiguous matches instead of opening an unrelated desktop behind identical crops.
func Resolve(hash string) string {
	if hash == "" {
		return ""
	}
	index.RLock()
	defer index.RUnlock()
	var match string
	for path := range index.hashes[hash] {
		if !Available(path) {
			continue
		}
		if match != "" {
			return ""
		}
		match = path
	}
	return match
}

// Remove drops only a screenshot's scene; clipboard history never owns this resource.
func Remove(path string) error {
	index.Lock()
	defer index.Unlock()
	removeIndexPath(path)
	if err := os.Remove(path + Suffix); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove screenshot scene: %w", err)
	}
	return nil
}

// removeIndexPath is called with the index lock held on scene replacement or removal.
func removeIndexPath(path string) {
	for hash, paths := range index.hashes {
		delete(paths, path)
		if len(paths) == 0 {
			delete(index.hashes, hash)
		}
	}
}
