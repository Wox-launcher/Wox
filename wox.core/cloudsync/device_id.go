package cloudsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"wox/util"

	"github.com/google/uuid"
)

type FileDeviceProvider struct {
	mu   sync.Mutex
	path string
}

func NewFileDeviceProvider(path string) *FileDeviceProvider {
	return &FileDeviceProvider{path: strings.TrimSpace(path)}
}

func (p *FileDeviceProvider) DeviceID(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	devicePath := p.path
	if devicePath == "" {
		devicePath = filepath.Join(util.GetLocation().GetWoxDataDirectory(), "device_id")
	}

	if value, ok := readDeviceID(devicePath); ok {
		return value, nil
	}

	id := uuid.NewString()
	if err := writeDeviceID(devicePath, id); err != nil {
		return "", err
	}

	return id, nil
}

func readDeviceID(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	id := strings.TrimSpace(string(data))
	if id == "" {
		return "", false
	}
	return id, true
}

func writeDeviceID(path string, value string) error {
	if value == "" {
		return fmt.Errorf("device id is empty")
	}
	return os.WriteFile(path, []byte(value), 0600)
}
