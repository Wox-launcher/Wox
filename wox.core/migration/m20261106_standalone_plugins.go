package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"wox/database"
	"wox/util"

	"gorm.io/gorm"
)

const (
	oldDictationPluginID = "a3f7b8c2-d1e4-4f6a-9b0c-7e2d1a5f8b3e"
	newDictationPluginID = "eaadbcea-c907-4efe-b09b-dfc56e1dc758"
	oldTimerPluginID     = "a3f8c2e1-9b4d-4e7a-8c1f-2d5e6a7b8c9d"
	newTimerPluginID     = "b18d674a-0ae0-4d20-bd11-c5268b069306"
)

func init() { Register(&standalonePluginsMigration{}) }

type standalonePluginsMigration struct{}

func (*standalonePluginsMigration) ID() string { return "20261106_standalone_timer_dictation" }
func (*standalonePluginsMigration) Description() string {
	return "Migrate built-in Timer and Dictation settings and models to standalone plugins."
}

// Up runs the settings and model migrations together, using a new ID for earlier development builds.
func (*standalonePluginsMigration) Up(ctx context.Context, tx *gorm.DB) error {
	location := util.GetLocation()
	cache, err := location.GetPluginCacheDirectory(newDictationPluginID)
	if err != nil {
		return err
	}
	return migrateStandalonePlugins(ctx, tx, location.GetDictationModelsDirectory(), filepath.Join(cache, "models"))
}

// migrateStandalonePlugins retries safely if filesystem work succeeds but the database transaction rolls back.
func migrateStandalonePlugins(ctx context.Context, tx *gorm.DB, source, destination string) error {
	if err := migrateStandaloneSettings(tx); err != nil {
		return err
	}
	if err := copyDictationModels(ctx, source, destination); err != nil {
		return err
	}
	return cleanupDictationModels(ctx, source, destination)
}

// migrateStandaloneSettings preserves the old settings and never overwrites standalone values.
func migrateStandaloneSettings(tx *gorm.DB) error {
	for _, pair := range [][2]string{{oldTimerPluginID, newTimerPluginID}, {oldDictationPluginID, newDictationPluginID}} {
		var rows []database.PluginSetting
		if err := tx.Where("plugin_id = ?", pair[0]).Order("key").Find(&rows).Error; err != nil {
			return err
		}
		// The old default action is stored inside actions; convert it before copying standalone fields.
		if pair[0] == oldDictationPluginID {
			for _, row := range rows {
				if row.Key != "actions" {
					continue
				}
				actions, defaults, err := splitDictationDefaultAction(row.Value)
				if err != nil {
					return err
				}
				for key, value := range defaults {
					if err := copyStandaloneSetting(tx, database.PluginSetting{PluginID: pair[1], Key: key, Value: value, IsLocal: row.IsLocal}); err != nil {
						return err
					}
				}
				row.Value = actions
				row.PluginID = pair[1]
				if err := copyStandaloneSetting(tx, row); err != nil {
					return err
				}
			}
		}
		for _, row := range rows {
			if pair[0] == oldDictationPluginID && row.Key == "actions" {
				continue
			}
			if pair[0] == oldDictationPluginID && (row.Key == "inputDevice" || strings.HasPrefix(row.Key, "inputDevice@")) && row.Value == "system" {
				row.Value = ""
			}
			row.PluginID = pair[1]
			if err := copyStandaloneSetting(tx, row); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyStandaloneSetting retains local-only flags and records synchronized values in the same transaction.
func copyStandaloneSetting(tx *gorm.DB, row database.PluginSetting) error {
	var existing database.PluginSetting
	err := tx.Where("plugin_id = ? AND key = ?", row.PluginID, row.Key).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err := tx.Create(&row).Error; err != nil {
		return err
	}
	if !row.IsLocal {
		return appendPluginSettingUpsertOplog(tx, row.PluginID, row.Key, row.Value)
	}
	return nil
}

// splitDictationDefaultAction maps the old combined default action to the standalone fields.
func splitDictationDefaultAction(value string) (string, map[string]string, error) {
	var actions []map[string]json.RawMessage
	if strings.TrimSpace(value) == "" {
		return value, nil, nil
	}
	if err := json.Unmarshal([]byte(value), &actions); err != nil {
		return "", nil, fmt.Errorf("migrate dictation actions: %w", err)
	}
	defaults := map[string]string{}
	remaining := make([]map[string]json.RawMessage, 0, len(actions))
	for _, action := range actions {
		var id, kind string
		_ = json.Unmarshal(action["id"], &id)
		_ = json.Unmarshal(action["type"], &kind)
		if id != "default" && kind != "default" {
			remaining = append(remaining, action)
			continue
		}
		defaults["defaultAIModel"] = ""
		defaults["defaultAIRefineEnabled"] = "false"
		for source, target := range map[string]string{"hotkey": "defaultHotkey", "model": "defaultAIModel"} {
			var v string
			if json.Unmarshal(action[source], &v) == nil {
				defaults[target] = v
			}
		}
		var enabled bool
		if json.Unmarshal(action["aiRefineEnabled"], &enabled) == nil {
			if enabled {
				defaults["defaultAIRefineEnabled"] = "true"
			} else {
				defaults["defaultAIRefineEnabled"] = "false"
			}
		}
	}
	data, err := json.Marshal(remaining)
	return string(data), defaults, err
}

// copyDictationModels keeps existing plugin data and the original files intact.
func copyDictationModels(ctx context.Context, source, destination string) error {
	entries, err := os.ReadDir(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "silero-vad" {
			continue
		}
		target := filepath.Join(destination, entry.Name())
		if _, err := os.Lstat(target); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.MkdirAll(destination, 0755); err != nil {
			return err
		}
		if err := copyDictationModel(filepath.Join(source, entry.Name()), target, filepath.Join(source, "silero-vad", "silero_vad.onnx")); err != nil {
			return fmt.Errorf("copy dictation model %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// copyDictationModel marks only supported, complete model layouts as usable by the plugin.
func copyDictationModel(source, target, vad string) error {
	stage, err := os.MkdirTemp(filepath.Dir(target), ".migrating-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := os.CopyFS(stage, os.DirFS(source)); err != nil {
		return err
	}
	for _, marker := range []string{".complete", ".wox-migrated"} {
		if err := os.Remove(filepath.Join(stage, marker)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	required := map[string][]string{
		"sherpa-onnx-qwen3-asr-0.6B-int8-2026-03-25":              {"conv_frontend.onnx", "encoder.int8.onnx", "decoder.int8.onnx", "tokenizer/vocab.json", "tokenizer/merges.txt"},
		"sherpa-onnx-sense-voice-zh-en-ja-ko-yue-int8-2024-07-17": {"model.int8.onnx", "tokens.txt"},
	}[filepath.Base(target)]
	valid := len(required) > 0
	for _, name := range required {
		info, err := os.Stat(filepath.Join(stage, filepath.FromSlash(name)))
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			valid = false
			break
		}
	}
	if valid {
		// The old VAD download had no checksum; only reuse the exact version required by the plugin.
		marker := ".wox-migrated"
		if f, err := os.Open(vad); err == nil {
			h := sha256.New()
			_, hashErr := io.Copy(h, f)
			closeErr := f.Close()
			if hashErr != nil || closeErr != nil {
				return errors.Join(hashErr, closeErr)
			}
			if hex.EncodeToString(h.Sum(nil)) == "9e2449e1087496d8d4caba907f23e0bd3f78d91fa552479bb9c23ac09cbb1fd6" {
				if err := copyDictationVAD(vad, filepath.Join(stage, "silero_vad.onnx")); err != nil {
					return err
				}
				marker = ".complete"
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.WriteFile(filepath.Join(stage, marker), []byte("legacy"), 0644); err != nil {
			return err
		}
	}
	return os.Rename(stage, target)
}

func copyDictationVAD(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	return errors.Join(copyErr, out.Close())
}

// cleanupDictationModels removes only source directories whose files are all present and identical.
func cleanupDictationModels(ctx context.Context, source, destination string) error {
	entries, err := os.ReadDir(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	remainingModel := false
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "silero-vad" {
			continue
		}
		oldPath, newPath := filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name())
		identical, err := identicalDictationModel(ctx, oldPath, newPath)
		if err != nil {
			return err
		}
		if !identical {
			remainingModel = true
			continue
		}
		if err := os.RemoveAll(oldPath); err != nil {
			return err
		}
	}
	if !remainingModel {
		if err := cleanupDictationVAD(source, destination); err != nil {
			return err
		}
	}
	// Hidden downloads and other unrelated files are intentionally left untouched.
	if err := os.Remove(source); err != nil && !errors.Is(err, os.ErrNotExist) {
		if entries, readErr := os.ReadDir(source); readErr != nil || len(entries) == 0 {
			return err
		}
	}
	return nil
}

var errDictationModelMismatch = errors.New("dictation model copy differs")

// identicalDictationModel refuses links, missing files, and changed data before removing anything.
func identicalDictationModel(ctx context.Context, source, destination string) (bool, error) {
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		info, err := os.Lstat(target)
		if errors.Is(err, os.ErrNotExist) {
			return errDictationModelMismatch
		}
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if !info.IsDir() {
				return errDictationModelMismatch
			}
			return nil
		}
		if !entry.Type().IsRegular() || !info.Mode().IsRegular() {
			return errDictationModelMismatch
		}
		same, err := sameFileContents(path, target)
		if err != nil {
			return err
		}
		if !same {
			return errDictationModelMismatch
		}
		return nil
	})
	if errors.Is(err, errDictationModelMismatch) {
		return false, nil
	}
	return err == nil, err
}

// cleanupDictationVAD removes the shared old VAD only when an identical copy exists.
func cleanupDictationVAD(source, destination string) error {
	oldDir := filepath.Join(source, "silero-vad")
	dirInfo, err := os.Lstat(oldDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !dirInfo.IsDir() {
		return err
	}
	oldVAD := filepath.Join(oldDir, "silero_vad.onnx")
	info, err := os.Lstat(oldVAD)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return err
	}
	entries, err := os.ReadDir(destination)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		newVAD := filepath.Join(destination, entry.Name(), "silero_vad.onnx")
		info, err := os.Lstat(newVAD)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		same, err := sameFileContents(oldVAD, newVAD)
		if err != nil {
			return err
		}
		if same {
			if err := os.Remove(oldVAD); err != nil {
				return err
			}
			// Remove the old VAD directory only if it is empty.
			if entries, err := os.ReadDir(filepath.Dir(oldVAD)); err == nil && len(entries) == 0 {
				return os.Remove(filepath.Dir(oldVAD))
			} else if err != nil {
				return err
			}
			break
		}
	}
	return nil
}
