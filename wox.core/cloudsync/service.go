package cloudsync

import (
	"context"
	"sync"
)

type Service struct {
	Manager        *CloudSyncManager
	Client         *CloudSyncHTTPClient
	KeyManager     *KeyManager
	DeviceProvider CloudSyncDeviceProvider
}

type ServiceStatus struct {
	Enabled   bool                `json:"enabled"`
	DeviceID  string              `json:"device_id,omitempty"`
	KeyStatus CloudSyncKeyStatus  `json:"key_status"`
	State     *CloudSyncStateView `json:"state,omitempty"`
}

type CloudSyncStateView struct {
	Cursor       string `json:"cursor"`
	LastPullTs   int64  `json:"last_pull_ts"`
	LastPushTs   int64  `json:"last_push_ts"`
	BackoffUntil int64  `json:"backoff_until"`
	RetryCount   int    `json:"retry_count"`
	LastError    string `json:"last_error"`
	Bootstrapped bool   `json:"bootstrapped"`
}

var (
	serviceMu sync.RWMutex
	service   *Service
)

func SetService(s *Service) {
	serviceMu.Lock()
	defer serviceMu.Unlock()
	service = s
}

func GetService() *Service {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	return service
}

func (s *Service) StartManager(ctx context.Context) {
	if s == nil || s.Manager == nil {
		return
	}
	s.Manager.Start(ctx)
}

func (s *Service) Status(ctx context.Context) ServiceStatus {
	status := ServiceStatus{Enabled: s != nil}
	if s == nil {
		return status
	}

	if s.DeviceProvider != nil {
		if deviceID, err := s.DeviceProvider.DeviceID(ctx); err == nil {
			status.DeviceID = deviceID
		}
	}

	if s.KeyManager != nil {
		status.KeyStatus = s.KeyManager.GetStatus(ctx)
	}

	if state, err := LoadCloudSyncState(ctx); err == nil && state != nil {
		status.State = &CloudSyncStateView{
			Cursor:       state.Cursor,
			LastPullTs:   state.LastPullTs,
			LastPushTs:   state.LastPushTs,
			BackoffUntil: state.BackoffUntil,
			RetryCount:   state.RetryCount,
			LastError:    state.LastError,
			Bootstrapped: state.Bootstrapped,
		}
	}

	return status
}
