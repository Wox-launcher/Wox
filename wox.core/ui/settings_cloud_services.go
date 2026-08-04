package ui

import (
	"context"
	"errors"
	"fmt"

	"wox/account"
	"wox/cloudsync"
	"wox/ui/contract"
)

// AccountStatus returns current local account state.
func (s *CoreServices) AccountStatus(ctx context.Context, sessionID string) (account.Status, error) {
	service := account.GetService()
	if service == nil {
		return account.Status{}, nil
	}
	return service.Status(uiServiceContext(ctx, sessionID)), nil
}

// CloudSyncStatus returns local sync state when the account is eligible and logged in.
func (s *CoreServices) CloudSyncStatus(ctx context.Context, sessionID string) (cloudsync.ServiceStatus, error) {
	ctx = uiServiceContext(ctx, sessionID)
	accountService := account.GetService()
	if accountService == nil || !accountService.Status(ctx).LoggedIn {
		return cloudsync.ServiceStatus{Enabled: false}, nil
	}
	service := cloudsync.GetService()
	if service == nil {
		return cloudsync.ServiceStatus{Enabled: false}, nil
	}
	return service.Status(ctx), nil
}

// CloudDevices refreshes and returns devices associated with the current sync identity.
func (s *CoreServices) CloudDevices(ctx context.Context, sessionID string) (cloudsync.CloudSyncDeviceListResponse, error) {
	ctx = uiServiceContext(ctx, sessionID)
	service := cloudsync.GetService()
	if service == nil || service.DeviceProvider == nil {
		return cloudsync.CloudSyncDeviceListResponse{}, errors.New("cloud sync is not configured")
	}
	deviceID, err := service.DeviceProvider.DeviceID(ctx)
	if err != nil {
		return cloudsync.CloudSyncDeviceListResponse{}, err
	}
	deviceClient := service.DeviceClient
	if deviceClient == nil {
		deviceClient = service.Client
	}
	if deviceClient == nil {
		return cloudsync.CloudSyncDeviceListResponse{}, errors.New("cloud sync is not configured")
	}
	if err := service.UpdateCurrentDevice(ctx); err != nil {
		logger.Warn(ctx, fmt.Sprintf("failed to update current cloud sync device before listing devices: %v", err))
	}
	response, err := deviceClient.ListDevices(ctx, cloudsync.CloudSyncDeviceListRequest{DeviceID: deviceID})
	if err != nil {
		return cloudsync.CloudSyncDeviceListResponse{}, err
	}
	if response == nil {
		return cloudsync.CloudSyncDeviceListResponse{}, errors.New("cloud sync device response is empty")
	}
	return *response, nil
}

// BillingPlan returns display pricing for account settings.
func (s *CoreServices) BillingPlan(ctx context.Context, sessionID string) (account.BillingPlan, error) {
	service := account.GetService()
	if service == nil {
		return account.BillingPlan{}, errors.New("account service is not configured")
	}
	return service.GetBillingPlan(uiServiceContext(ctx, sessionID))
}

// LoginAccount authenticates an existing account using the account API locale set.
func (s *CoreServices) LoginAccount(ctx context.Context, sessionID string, email string, password string, lang string) (account.ActionResult, error) {
	service, err := configuredAccountService()
	if err != nil {
		return account.ActionResult{}, err
	}
	return service.Login(uiServiceContext(ctx, sessionID), email, password, accountRequestLang(lang))
}

// RegisterAccount creates an account using the account API locale set.
func (s *CoreServices) RegisterAccount(ctx context.Context, sessionID string, email string, password string, lang string) (account.ActionResult, error) {
	service, err := configuredAccountService()
	if err != nil {
		return account.ActionResult{}, err
	}
	return service.Register(uiServiceContext(ctx, sessionID), email, password, accountRequestLang(lang))
}

// VerifyAccountEmail completes account verification.
func (s *CoreServices) VerifyAccountEmail(ctx context.Context, sessionID string, email string, code string, lang string) (account.ActionResult, error) {
	service, err := configuredAccountService()
	if err != nil {
		return account.ActionResult{}, err
	}
	return service.VerifyEmail(uiServiceContext(ctx, sessionID), email, code, accountRequestLang(lang))
}

// LogoutAccount clears account credentials and best-effort local sync state.
func (s *CoreServices) LogoutAccount(ctx context.Context, sessionID string) error {
	ctx = uiServiceContext(ctx, sessionID)
	service := account.GetService()
	if service == nil {
		return nil
	}
	if err := service.Logout(ctx); err != nil {
		return err
	}
	if cloudService := cloudsync.GetService(); cloudService != nil {
		if err := cloudService.ResetLocalState(ctx); err != nil {
			logger.Warn(ctx, fmt.Sprintf("failed to reset cloud sync state during logout: %v", err))
		}
	}
	return nil
}

// ResendAccountVerification requests another email verification code.
func (s *CoreServices) ResendAccountVerification(ctx context.Context, sessionID string, email string, lang string) error {
	service, err := configuredAccountService()
	if err != nil {
		return err
	}
	return service.ResendVerification(uiServiceContext(ctx, sessionID), email, accountRequestLang(lang))
}

// RequestAccountPasswordReset starts the password reset flow.
func (s *CoreServices) RequestAccountPasswordReset(ctx context.Context, sessionID string, email string, lang string) error {
	service, err := configuredAccountService()
	if err != nil {
		return err
	}
	return service.RequestPasswordReset(uiServiceContext(ctx, sessionID), email, accountRequestLang(lang))
}

// ConfirmAccountPasswordReset completes the password reset flow.
func (s *CoreServices) ConfirmAccountPasswordReset(ctx context.Context, sessionID string, token string, password string, lang string) error {
	service, err := configuredAccountService()
	if err != nil {
		return err
	}
	return service.ConfirmPasswordReset(uiServiceContext(ctx, sessionID), token, password, accountRequestLang(lang))
}

// ChangeAccountPassword updates the password for the current account.
func (s *CoreServices) ChangeAccountPassword(ctx context.Context, sessionID string, currentPassword string, newPassword string, lang string) error {
	service, err := configuredAccountService()
	if err != nil {
		return err
	}
	return service.ChangePassword(uiServiceContext(ctx, sessionID), currentPassword, newPassword, accountRequestLang(lang))
}

// BillingSession creates the requested hosted billing session.
func (s *CoreServices) BillingSession(ctx context.Context, sessionID string, kind contract.BillingSessionKind) (account.BillingSession, error) {
	if kind != contract.BillingSessionCheckout && kind != contract.BillingSessionPortal {
		return account.BillingSession{}, fmt.Errorf("unsupported billing session kind %q", kind)
	}
	service, err := configuredAccountService()
	if err != nil {
		return account.BillingSession{}, err
	}
	ctx = uiServiceContext(ctx, sessionID)
	if kind == contract.BillingSessionCheckout {
		return service.CreateCheckoutSession(ctx)
	}
	return service.CreatePortalSession(ctx)
}

// CloudBootstrapStatus returns the remote data and key state needed by recovery UI.
func (s *CoreServices) CloudBootstrapStatus(ctx context.Context, sessionID string) (contract.CloudBootstrapStatus, error) {
	return resolveSyncBootstrapStatus(uiServiceContext(ctx, sessionID))
}

// StartCloudBootstrap initializes or restores the sync key and schedules the first transfer.
func (s *CoreServices) StartCloudBootstrap(ctx context.Context, sessionID string, recoveryCode string) error {
	return startSyncBootstrap(uiServiceContext(ctx, sessionID), recoveryCode)
}

// PushCloudChanges flushes pending local changes to cloud sync.
func (s *CoreServices) PushCloudChanges(ctx context.Context, sessionID string) error {
	ctx = uiServiceContext(ctx, sessionID)
	service := cloudsync.GetService()
	if service == nil || service.Manager == nil {
		return errors.New("cloud sync is not configured")
	}
	startCloudSyncManagerIfSyncEnabled(ctx, service)
	service.Manager.PushPending(ctx, "manual")
	return nil
}

// PullCloudChanges fetches remote changes into local cloud sync state.
func (s *CoreServices) PullCloudChanges(ctx context.Context, sessionID string) error {
	ctx = uiServiceContext(ctx, sessionID)
	service := cloudsync.GetService()
	if service == nil || service.Manager == nil {
		return errors.New("cloud sync is not configured")
	}
	startCloudSyncManagerIfSyncEnabled(ctx, service)
	service.Manager.Pull(ctx, "manual")
	return nil
}

// SyncCloud performs the manual push-then-pull sequence used by settings.
func (s *CoreServices) SyncCloud(ctx context.Context, sessionID string) error {
	if err := s.PushCloudChanges(ctx, sessionID); err != nil {
		return err
	}
	return s.PullCloudChanges(ctx, sessionID)
}

// JoinCloudDevice restores the current device after it was revoked.
func (s *CoreServices) JoinCloudDevice(ctx context.Context, sessionID string) error {
	ctx = uiServiceContext(ctx, sessionID)
	if err := ensureSyncBootstrapAllowed(ctx); err != nil {
		return err
	}
	service := cloudsync.GetService()
	if service == nil {
		return errors.New("cloud sync is not configured")
	}
	if err := service.JoinCurrentDevice(ctx); err != nil {
		return err
	}
	startCloudSyncManagerIfSyncEnabled(ctx, service)
	return nil
}

// RevokeCloudDevice revokes one non-current device and returns the server response.
func (s *CoreServices) RevokeCloudDevice(ctx context.Context, sessionID string, targetDeviceID string) (*cloudsync.CloudSyncDeviceRevokeResponse, error) {
	ctx = uiServiceContext(ctx, sessionID)
	service := cloudsync.GetService()
	if service == nil || service.DeviceProvider == nil {
		return nil, errors.New("cloud sync is not configured")
	}
	deviceID, err := service.DeviceProvider.DeviceID(ctx)
	if err != nil {
		return nil, err
	}
	deviceClient := service.DeviceClient
	if deviceClient == nil {
		deviceClient = service.Client
	}
	if deviceClient == nil {
		return nil, errors.New("cloud sync is not configured")
	}
	return deviceClient.RevokeDevice(ctx, cloudsync.CloudSyncDeviceRevokeRequest{DeviceID: deviceID, TargetDeviceID: targetDeviceID})
}

func configuredAccountService() (*account.Service, error) {
	service := account.GetService()
	if service == nil {
		return nil, errors.New("account service is not configured")
	}
	return service, nil
}
