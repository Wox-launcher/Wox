package migration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"wox/util"

	"gorm.io/gorm"
)

func init() {
	Register(&moveDictationModelsMigration{})
}

type moveDictationModelsMigration struct{}

func (m *moveDictationModelsMigration) ID() string {
	return "20260715_move_dictation_models"
}

func (m *moveDictationModelsMigration) Description() string {
	return "Move dictation models from the legacy feature directory into the shared models directory."
}

// Up moves the legacy model directory only when the new destination is empty.
func (m *moveDictationModelsMigration) Up(ctx context.Context, _ *gorm.DB) error {
	location := util.GetLocation()
	legacyModelsDir := filepath.Join(location.GetLegacyDictationDirectory(), "models")
	modelsDir := location.GetDictationModelsDirectory()
	if !util.IsDirExists(legacyModelsDir) || util.IsDirExists(modelsDir) {
		return nil
	}
	if err := os.Rename(legacyModelsDir, modelsDir); err != nil {
		return fmt.Errorf("move legacy dictation models: %w", err)
	}
	util.GetLogger().Info(ctx, "migrated legacy dictation models to the shared models directory")
	return nil
}
