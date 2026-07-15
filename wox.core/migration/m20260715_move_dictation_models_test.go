package migration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"wox/util"
)

func TestMoveDictationModelsMigration(t *testing.T) {
	woxDataDir := t.TempDir()
	t.Setenv(util.TestWoxDataDirEnv, woxDataDir)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(woxDataDir, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}

	legacyModelsDir := filepath.Join(util.GetLocation().GetLegacyDictationDirectory(), "models", "model-a")
	if err := os.MkdirAll(legacyModelsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyModelsDir, "model.onnx"), []byte("model"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := (&moveDictationModelsMigration{}).Up(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if !util.IsFileExists(filepath.Join(util.GetLocation().GetDictationModelsDirectory(), "model-a", "model.onnx")) {
		t.Fatal("expected the legacy dictation model to move to the shared models directory")
	}
}
