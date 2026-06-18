package cloudsync

import (
	"context"
	"fmt"
	"sync"
	"wox/util"
)

type Service struct {
	Manager        *CloudSyncManager
	Client         *CloudSyncHTTPClient
	DeviceClient   CloudSyncDeviceClient
	KeyManager     *KeyManager
	DeviceProvider CloudSyncDeviceProvider
	HistoryStore   CloudSyncHistoryStore
}

type ServiceStatus struct {
	Enabled      bool                `json:"enabled"`
	DeviceID     string              `json:"device_id,omitempty"`
	KeyStatus    CloudSyncKeyStatus  `json:"key_status"`
	State        *CloudSyncStateView `json:"state,omitempty"`
	Progress     *CloudSyncProgress  `json:"progress,omitempty"`
	PendingCount int                 `json:"pending_count"`
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

// UpdateCurrentDevice refreshes server-side metadata for the local device without requiring a sync pass.
func (s *Service) UpdateCurrentDevice(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("cloud sync is not configured")
	}
	client := s.DeviceClient
	if client == nil {
		client = s.Client
	}
	if client == nil || s.DeviceProvider == nil {
		return fmt.Errorf("cloud sync is not configured")
	}

	deviceID, err := s.DeviceProvider.DeviceID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get device id: %w", err)
	}
	_, err = client.UpdateDevice(ctx, CloudSyncDeviceUpdateRequest{
		DeviceID:   deviceID,
		DeviceName: resolveDeviceName(),
		Platform:   util.GetCurrentPlatform(),
	})
	if err != nil {
		return fmt.Errorf("cloud sync device update failed: %w", err)
	}
	return nil
}

// ResetLocalState clears sync runtime and account-scoped local state during logout or account-server changes.
func (s *Service) ResetLocalState(ctx context.Context) error {
	if s == nil {
		return nil
	}
	var resetErr error
	if s.Manager != nil {
		s.Manager.Stop(ctx)
	}
	if s.KeyManager != nil {
		if err := s.KeyManager.ClearLocalKey(ctx); err != nil {
			resetErr = err
		}
	}
	if err := ResetCloudSyncState(ctx); err != nil {
		return err
	}
	return resetErr
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

	if s.Manager != nil {
		status.PendingCount = s.Manager.countPendingOplogs(ctx)
		progress := s.Manager.Progress()
		if progress.Active {
			status.Progress = &progress
		}
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
