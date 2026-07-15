package cloudsync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"wox/database"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const deviceIdentityID = 1

type DatabaseDeviceProvider struct {
	mu sync.Mutex
}

func NewDatabaseDeviceProvider() *DatabaseDeviceProvider {
	return &DatabaseDeviceProvider{}
}

func (p *DatabaseDeviceProvider) DeviceID(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	db := database.GetDB()
	if db == nil {
		return "", fmt.Errorf("database not initialized")
	}

	var identity database.DeviceIdentity
	err := db.First(&identity, deviceIdentityID).Error
	if err == nil && strings.TrimSpace(identity.DeviceID) != "" {
		return identity.DeviceID, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", fmt.Errorf("load device identity: %w", err)
	}

	id := uuid.NewString()
	identity = database.DeviceIdentity{ID: deviceIdentityID, DeviceID: id}
	if err := db.Save(&identity).Error; err != nil {
		return "", fmt.Errorf("save device identity: %w", err)
	}

	return id, nil
}
