package ui

import (
	"context"
	"fmt"
	"strings"

	"wox/account"
	"wox/cloudsync"
	"wox/ui/contract"
	"wox/util"
)

func resolveSyncBootstrapStatus(ctx context.Context) (contract.CloudBootstrapStatus, error) {
	if err := ensureSyncBootstrapAllowed(ctx); err != nil {
		return contract.CloudBootstrapStatus{}, err
	}
	service := cloudsync.GetService()
	if service == nil || service.Manager == nil || service.KeyManager == nil {
		return contract.CloudBootstrapStatus{}, fmt.Errorf("cloud sync is not configured")
	}

	hasRemoteData, err := service.Manager.HasRemoteSnapshotData(ctx)
	if err != nil {
		return contract.CloudBootstrapStatus{}, err
	}
	remoteKeyStatus, err := service.KeyManager.RemoteStatus(ctx)
	if err != nil {
		return contract.CloudBootstrapStatus{}, err
	}
	return contract.CloudBootstrapStatus{HasRemoteData: hasRemoteData, HasRemoteKey: remoteKeyStatus.Available}, nil
}

func startSyncBootstrap(ctx context.Context, recoveryCode string) error {
	status, err := resolveSyncBootstrapStatus(ctx)
	if err != nil {
		return err
	}
	service := cloudsync.GetService()
	if service == nil || service.Manager == nil || service.KeyManager == nil {
		return fmt.Errorf("cloud sync is not configured")
	}

	if status.HasRemoteKey {
		// A failed snapshot apply does not invalidate the restored key, so retries must not request the recovery code again.
		if !service.KeyManager.GetStatus(ctx).Available {
			if strings.TrimSpace(recoveryCode) == "" {
				return fmt.Errorf("recovery_code is empty")
			}
			if _, err := service.KeyManager.FetchWithRecoveryCode(ctx, recoveryCode); err != nil {
				return err
			}
		}
	} else {
		if status.HasRemoteData {
			return fmt.Errorf("cloud sync key is missing")
		}
		if strings.TrimSpace(recoveryCode) == "" {
			return fmt.Errorf("recovery_code is empty")
		}
		if _, err := service.KeyManager.InitWithRecoveryCode(ctx, recoveryCode, ""); err != nil {
			return err
		}
	}
	cloudsync.MarkCloudSyncBootstrapPending(ctx)

	if accountService := account.GetService(); accountService != nil {
		if err := accountService.SetSyncEnabled(ctx, true); err != nil {
			return err
		}
	}
	if status.HasRemoteData {
		scheduleCloudSyncBootstrapRestore(ctx, service)
		return nil
	}
	startCloudSyncManagerIfSyncEnabled(ctx, service)
	scheduleCloudSyncBootstrapInitialPush(ctx, service)
	return nil
}

// scheduleCloudSyncBootstrapRestore restores remote data before starting the regular sync manager.
func scheduleCloudSyncBootstrapRestore(ctx context.Context, service *cloudsync.Service) {
	util.Go(ctx, "cloud sync bootstrap restore", func() {
		if service == nil || service.Manager == nil {
			return
		}
		if err := service.Manager.RestoreSnapshot(ctx); err != nil {
			cloudsync.RecordCloudSyncBootstrapFailure(ctx, err)
			util.GetLogger().Error(ctx, fmt.Sprintf("cloud sync bootstrap restore failed: %v", err))
			return
		}
		startCloudSyncManagerIfSyncEnabled(ctx, service)
	})
}

// scheduleCloudSyncBootstrapInitialPush performs the first local-to-cloud push after the dialog can close.
func scheduleCloudSyncBootstrapInitialPush(ctx context.Context, service *cloudsync.Service) {
	util.Go(ctx, "cloud sync bootstrap initial push", func() {
		if service == nil || service.Manager == nil {
			return
		}
		service.Manager.PushLocalSnapshot(ctx, "bootstrap")
		state, err := cloudsync.LoadCloudSyncState(ctx)
		if err != nil {
			cloudsync.RecordCloudSyncBootstrapFailure(ctx, err)
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to load cloud sync bootstrap state: %v", err))
			return
		}
		if state.LastError != "" {
			return
		}
		cloudsync.MarkCloudSyncBootstrapComplete(ctx)
		startCloudSyncManagerIfSyncEnabled(ctx, service)
	})
}

func ensureSyncBootstrapAllowed(ctx context.Context) error {
	accountService := account.GetService()
	accountStatus := account.Status{}
	if accountService != nil {
		accountStatus = accountService.Status(ctx)
	}
	if accountService == nil || !accountStatus.LoggedIn {
		return fmt.Errorf("account is not logged in")
	}
	if !accountStatus.SyncEligible {
		return fmt.Errorf("subscription_required")
	}
	return nil
}

// startCloudSyncManagerAfterUIReady starts the scheduler only after the UI can apply settings from a scheduled pull.
func startCloudSyncManagerAfterUIReady(ctx context.Context) {
	startCloudSyncManagerIfSyncEnabled(ctx, cloudsync.GetService())
}

// startCloudSyncManagerIfSyncEnabled starts a configured scheduler after account and bootstrap checks pass.
func startCloudSyncManagerIfSyncEnabled(ctx context.Context, service *cloudsync.Service) {
	if service == nil || service.Manager == nil {
		return
	}
	accountService := account.GetService()
	if accountService == nil {
		return
	}
	accountStatus := accountService.Status(ctx)
	if !accountStatus.LoggedIn || !accountStatus.SyncEligible || !accountStatus.SyncEnabled {
		return
	}
	if service.KeyManager == nil || !service.KeyManager.GetStatus(ctx).Available {
		return
	}
	state, err := cloudsync.LoadCloudSyncState(ctx)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to load cloud sync state before starting scheduler: %v", err))
		return
	}
	if !state.Bootstrapped {
		return
	}
	service.StartManager(ctx)
}
