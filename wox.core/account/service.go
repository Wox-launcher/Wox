package account

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"wox/cloudsync"
	"wox/database"
	"wox/util"

	"gorm.io/gorm"
)

const (
	defaultBaseURL    = "https://sync.woxlauncher.com"
	accountStateID    = 1
	keyringService    = "wox.account"
	accessTokenKey    = "access_token"
	refreshTokenKey   = "refresh_token"
	accessExpiresKey  = "access_expires_at"
	refreshExpiresKey = "refresh_expires_at"

	tokenRefreshLeadTime   = 2 * time.Minute
	tokenRefreshIdleDelay  = 5 * time.Minute
	tokenRefreshRetryDelay = time.Minute
)

type Service struct {
	baseURL         string
	baseURLMu       sync.RWMutex
	keyring         cloudsync.KeyringStore
	mu              sync.Mutex
	refreshLoopOnce sync.Once
	refreshWake     chan struct{}
}

type Status struct {
	LoggedIn                     bool   `json:"logged_in"`
	UserID                       string `json:"user_id"`
	Email                        string `json:"email"`
	EmailVerified                bool   `json:"email_verified"`
	SubscriptionStatus           string `json:"subscription_status"`
	SubscriptionCurrentPeriodEnd int64  `json:"subscription_current_period_end"`
	SyncEligible                 bool   `json:"sync_eligible"`
	SyncEnabled                  bool   `json:"sync_enabled"`
	SessionExpired               bool   `json:"session_expired"`
}

type User struct {
	ID                           string `json:"id"`
	Email                        string `json:"email"`
	EmailVerified                bool   `json:"email_verified"`
	SubscriptionStatus           string `json:"subscription_status"`
	SubscriptionCurrentPeriodEnd int64  `json:"subscription_current_period_end"`
	SyncEligible                 bool   `json:"sync_eligible"`
}

type AuthResponse struct {
	User             User   `json:"user"`
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	AccessExpiresAt  int64  `json:"access_expires_at"`
	RefreshExpiresAt int64  `json:"refresh_expires_at"`
}

type ActionResult struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Email     string `json:"email,omitempty"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
}

type BillingSession struct {
	URL string `json:"url"`
}

type emailVerificationRequired struct {
	Email     string `json:"email"`
	ExpiresAt int64  `json:"expires_at"`
}

type responseEnvelope struct {
	Status  int             `json:"status"`
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

var (
	serviceMu              sync.RWMutex
	service                *Service
	errAccountUnauthorized = errors.New("unauthorized")
)

func NewService(baseURL ...string) *Service {
	resolvedBaseURL := defaultBaseURL
	if len(baseURL) > 0 && strings.TrimSpace(baseURL[0]) != "" {
		resolvedBaseURL = normalizeBaseURL(baseURL[0])
	}

	return &Service{
		baseURL: resolvedBaseURL,
		keyring: cloudsync.NewOSKeyringStore(
			keyringService,
		),
		refreshWake: make(chan struct{}, 1),
	}
}

func (s *Service) SetBaseURL(baseURL string) {
	if s == nil {
		return
	}
	s.baseURLMu.Lock()
	defer s.baseURLMu.Unlock()
	s.baseURL = normalizeBaseURL(baseURL)
}

func (s *Service) BaseURL() string {
	if s == nil {
		return defaultBaseURL
	}
	s.baseURLMu.RLock()
	defer s.baseURLMu.RUnlock()
	return s.baseURL
}

func normalizeBaseURL(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return defaultBaseURL
	}
	return trimmed
}

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

func (s *Service) Status(ctx context.Context) Status {
	state := s.loadState(ctx)
	return Status{
		LoggedIn:                     state.UserID != "",
		UserID:                       state.UserID,
		Email:                        state.Email,
		EmailVerified:                state.EmailVerified,
		SubscriptionStatus:           normalizeSubscriptionStatus(state.SubscriptionStatus),
		SubscriptionCurrentPeriodEnd: state.SubscriptionCurrentPeriodEnd,
		SyncEligible:                 state.SyncEligible,
		SyncEnabled:                  state.SyncEnabled,
		SessionExpired:               state.SessionExpired,
	}
}

func (s *Service) Register(ctx context.Context, email string, password string, lang string) (ActionResult, error) {
	envelope, err := s.postEnvelope(ctx, "/v1/account/register", map[string]string{"email": email, "password": password, "lang": lang}, "")
	if err != nil {
		return ActionResult{}, err
	}
	return s.actionResultFromEnvelope(ctx, envelope)
}

func (s *Service) VerifyEmail(ctx context.Context, email string, code string, lang string) (ActionResult, error) {
	var resp AuthResponse
	envelope, err := s.postEnvelope(ctx, "/v1/account/verify-email", map[string]string{"email": email, "code": code, "lang": lang}, "")
	if err != nil {
		return ActionResult{}, err
	}
	if envelope.Code != "ok" {
		return s.actionResultFromEnvelope(ctx, envelope)
	}
	if err := decodeEnvelopeData(envelope, &resp); err != nil {
		return ActionResult{}, err
	}
	if err := s.saveTokens(ctx, resp); err != nil {
		return ActionResult{}, err
	}
	if err := s.saveState(ctx, resp.User, s.loadState(ctx).SyncEnabled, false); err != nil {
		return ActionResult{}, err
	}
	s.notifyTokenRefreshLoop()
	return ActionResult{Code: envelope.Code, Message: envelope.Message}, nil
}

func (s *Service) Login(ctx context.Context, email string, password string, lang string) (ActionResult, error) {
	envelope, err := s.postEnvelope(ctx, "/v1/account/login", map[string]string{"email": email, "password": password, "lang": lang}, "")
	if err != nil {
		return ActionResult{}, err
	}
	if envelope.Code == "need_verify_email" {
		return s.actionResultFromEnvelope(ctx, envelope)
	}
	var resp AuthResponse
	if err := decodeEnvelopeData(envelope, &resp); err != nil {
		return ActionResult{}, err
	}
	if err := s.saveTokens(ctx, resp); err != nil {
		return ActionResult{}, err
	}
	if err := s.saveState(ctx, resp.User, s.loadState(ctx).SyncEnabled, false); err != nil {
		return ActionResult{}, err
	}
	s.notifyTokenRefreshLoop()
	return ActionResult{Code: envelope.Code, Message: envelope.Message}, nil
}

func (s *Service) ResendVerification(ctx context.Context, email string, lang string) error {
	return s.post(ctx, "/v1/account/resend-verification", map[string]string{"email": email, "lang": lang}, nil, "")
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string, lang string) error {
	return s.post(ctx, "/v1/account/password-reset/request", map[string]string{"email": email, "lang": lang}, nil, "")
}

func (s *Service) ConfirmPasswordReset(ctx context.Context, token string, password string, lang string) error {
	return s.post(ctx, "/v1/account/password-reset/confirm", map[string]string{"token": token, "password": password, "lang": lang}, nil, "")
}

func (s *Service) ChangePassword(ctx context.Context, currentPassword string, newPassword string, lang string) error {
	return s.postAuthenticated(ctx, "/v1/account/change-password", map[string]string{"current_password": currentPassword, "new_password": newPassword, "lang": lang}, nil)
}

func (s *Service) RefreshAccount(ctx context.Context) error {
	var resp struct {
		User User `json:"user"`
	}
	if err := s.getAuthenticated(ctx, "/v1/account/me", &resp); err != nil {
		return err
	}
	return s.saveState(ctx, resp.User, s.loadState(ctx).SyncEnabled, false)
}

func (s *Service) CreateCheckoutSession(ctx context.Context) (BillingSession, error) {
	var resp BillingSession
	if err := s.postAuthenticated(ctx, "/v1/billing/checkout", map[string]any{}, &resp); err != nil {
		return BillingSession{}, err
	}
	return resp, nil
}

func (s *Service) CreatePortalSession(ctx context.Context) (BillingSession, error) {
	var resp BillingSession
	if err := s.postAuthenticated(ctx, "/v1/billing/portal", map[string]any{}, &resp); err != nil {
		return BillingSession{}, err
	}
	return resp, nil
}

func (s *Service) Logout(ctx context.Context) error {
	token, _ := s.AccessToken(ctx)
	_ = s.post(ctx, "/v1/account/logout", map[string]any{}, nil, token)
	_ = s.clearTokens(ctx)
	state := s.loadState(ctx)
	state.UserID = ""
	state.Email = ""
	state.EmailVerified = false
	state.SubscriptionStatus = ""
	state.SubscriptionCurrentPeriodEnd = 0
	state.SyncEligible = false
	state.SyncEnabled = false
	state.SessionExpired = false
	return saveAccountState(ctx, state)
}

// ResetLocalSession clears account state when the configured account server changes.
func (s *Service) ResetLocalSession(ctx context.Context) error {
	if s == nil {
		return nil
	}
	_ = s.clearTokens(ctx)
	state := s.loadState(ctx)
	state.UserID = ""
	state.Email = ""
	state.EmailVerified = false
	state.SubscriptionStatus = ""
	state.SubscriptionCurrentPeriodEnd = 0
	state.SyncEligible = false
	state.SyncEnabled = false
	state.SessionExpired = false
	return saveAccountState(ctx, state)
}

func (s *Service) SetSyncEnabled(ctx context.Context, enabled bool) error {
	state := s.loadState(ctx)
	state.SyncEnabled = enabled
	return saveAccountState(ctx, state)
}

func (s *Service) MarkSessionExpired(ctx context.Context) {
	state := s.loadState(ctx)
	state.SessionExpired = true
	_ = saveAccountState(ctx, state)
}

func (s *Service) StartTokenRefresh(ctx context.Context) {
	if s == nil {
		return
	}
	s.refreshLoopOnce.Do(func() {
		util.Go(ctx, "account token refresh loop", func() {
			s.runTokenRefreshLoop(ctx)
		})
	})
}

func (s *Service) notifyTokenRefreshLoop() {
	if s == nil || s.refreshWake == nil {
		return
	}
	select {
	case s.refreshWake <- struct{}{}:
	default:
	}
}

func (s *Service) runTokenRefreshLoop(ctx context.Context) {
	for {
		delay := s.nextTokenRefreshDelay(ctx)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-s.refreshWake:
			timer.Stop()
			continue
		case <-timer.C:
		}

		state := s.loadState(ctx)
		if state.UserID == "" || state.SessionExpired {
			continue
		}
		if _, err := s.RefreshAccessToken(ctx); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to refresh account token: %v", err))
			s.waitTokenRefreshRetry(ctx)
		}
	}
}

func (s *Service) nextTokenRefreshDelay(ctx context.Context) time.Duration {
	state := s.loadState(ctx)
	if state.UserID == "" || state.SessionExpired {
		return tokenRefreshIdleDelay
	}

	expiresAt, err := s.tokenExpiresAt(ctx, accessExpiresKey)
	if err != nil || expiresAt.IsZero() {
		return tokenRefreshIdleDelay
	}
	refreshAt := expiresAt.Add(-tokenRefreshLeadTime)
	delay := time.Until(refreshAt)
	if delay < 0 {
		return 0
	}
	return delay
}

func (s *Service) waitTokenRefreshRetry(ctx context.Context) {
	timer := time.NewTimer(tokenRefreshRetryDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-s.refreshWake:
	case <-timer.C:
	}
}

func (s *Service) tokenExpiresAt(ctx context.Context, key string) (time.Time, error) {
	raw, err := s.keyring.Get(ctx, key)
	if err != nil {
		return time.Time{}, err
	}
	timestamp, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	if timestamp > 10000000000 {
		return time.UnixMilli(timestamp), nil
	}
	return time.Unix(timestamp, 0), nil
}

func (s *Service) AccessToken(ctx context.Context) (string, error) {
	if s == nil {
		return "", fmt.Errorf("account service not configured")
	}
	token, err := s.keyring.Get(ctx, accessTokenKey)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) RefreshAccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	refreshToken, err := s.keyring.Get(ctx, refreshTokenKey)
	if err != nil {
		s.MarkSessionExpired(ctx)
		return "", err
	}

	var resp AuthResponse
	if err := s.post(ctx, "/v1/account/refresh", map[string]string{"refresh_token": refreshToken}, &resp, ""); err != nil {
		s.MarkSessionExpired(ctx)
		return "", err
	}
	if err := s.saveTokens(ctx, resp); err != nil {
		return "", err
	}
	if err := s.saveState(ctx, resp.User, s.loadState(ctx).SyncEnabled, false); err != nil {
		return "", err
	}
	return resp.AccessToken, nil
}

func (s *Service) saveTokens(ctx context.Context, resp AuthResponse) error {
	if err := s.keyring.Set(ctx, accessTokenKey, resp.AccessToken); err != nil {
		return err
	}
	if err := s.keyring.Set(ctx, refreshTokenKey, resp.RefreshToken); err != nil {
		return err
	}
	if err := s.keyring.Set(ctx, accessExpiresKey, fmt.Sprintf("%d", resp.AccessExpiresAt)); err != nil {
		return err
	}
	return s.keyring.Set(ctx, refreshExpiresKey, fmt.Sprintf("%d", resp.RefreshExpiresAt))
}

func (s *Service) clearTokens(ctx context.Context) error {
	_ = s.keyring.Delete(ctx, accessTokenKey)
	_ = s.keyring.Delete(ctx, refreshTokenKey)
	_ = s.keyring.Delete(ctx, accessExpiresKey)
	_ = s.keyring.Delete(ctx, refreshExpiresKey)
	return nil
}

func (s *Service) saveState(ctx context.Context, user User, syncEnabled bool, sessionExpired bool) error {
	return saveAccountState(ctx, database.AccountState{
		ID:                           accountStateID,
		UserID:                       user.ID,
		Email:                        user.Email,
		EmailVerified:                user.EmailVerified,
		SubscriptionStatus:           normalizeSubscriptionStatus(user.SubscriptionStatus),
		SubscriptionCurrentPeriodEnd: user.SubscriptionCurrentPeriodEnd,
		SyncEligible:                 user.SyncEligible,
		SyncEnabled:                  syncEnabled,
		SessionExpired:               sessionExpired,
	})
}

func (s *Service) loadState(ctx context.Context) database.AccountState {
	state, err := loadAccountState(ctx)
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to load account state: %v", err))
		return database.AccountState{ID: accountStateID}
	}
	return state
}

func (s *Service) post(ctx context.Context, path string, body any, target any, accessToken string) error {
	envelope, err := s.postEnvelope(ctx, path, body, accessToken)
	if err != nil {
		return err
	}
	if target == nil {
		return nil
	}
	return decodeEnvelopeData(envelope, target)
}

func (s *Service) postAuthenticated(ctx context.Context, path string, body any, target any) error {
	token, err := s.AccessToken(ctx)
	if err != nil {
		return err
	}
	err = s.post(ctx, path, body, target, token)
	if !errors.Is(err, errAccountUnauthorized) {
		return err
	}
	token, refreshErr := s.RefreshAccessToken(ctx)
	if refreshErr != nil {
		return err
	}
	return s.post(ctx, path, body, target, token)
}

func (s *Service) get(ctx context.Context, path string, target any, accessToken string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.BaseURL()+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	envelope, err := s.doEnvelopeRequest(ctx, req)
	if err != nil {
		return err
	}
	return decodeEnvelopeData(envelope, target)
}

func (s *Service) getAuthenticated(ctx context.Context, path string, target any) error {
	token, err := s.AccessToken(ctx)
	if err != nil {
		return err
	}
	err = s.get(ctx, path, target, token)
	if !errors.Is(err, errAccountUnauthorized) {
		return err
	}
	token, refreshErr := s.RefreshAccessToken(ctx)
	if refreshErr != nil {
		return err
	}
	return s.get(ctx, path, target, token)
}

func (s *Service) postEnvelope(ctx context.Context, path string, body any, accessToken string) (responseEnvelope, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return responseEnvelope{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL()+path, bytes.NewReader(payload))
	if err != nil {
		return responseEnvelope{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	return s.doEnvelopeRequest(ctx, req)
}

func (s *Service) doEnvelopeRequest(ctx context.Context, req *http.Request) (responseEnvelope, error) {
	resp, err := util.GetHTTPClient(ctx).Do(req)
	if err != nil {
		return responseEnvelope{}, err
	}
	defer resp.Body.Close()

	responsePayload, err := io.ReadAll(resp.Body)
	if err != nil {
		return responseEnvelope{}, err
	}

	var envelope responseEnvelope
	envelopeErr := json.Unmarshal(responsePayload, &envelope)
	if resp.StatusCode >= 400 {
		if resp.StatusCode == http.StatusUnauthorized || (envelopeErr == nil && envelope.Code == "unauthorized") {
			return responseEnvelope{}, errAccountUnauthorized
		}
		if envelopeErr == nil && envelope.Message != "" {
			return responseEnvelope{}, errors.New(envelope.Message)
		}
		if envelopeErr == nil && envelope.Code != "" {
			return responseEnvelope{}, errors.New(envelope.Code)
		}
		return responseEnvelope{}, fmt.Errorf("account request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(responsePayload)))
	}
	if envelopeErr == nil && envelope.Code != "" {
		return envelope, nil
	}
	return responseEnvelope{Status: resp.StatusCode, Code: "ok", Message: "OK", Data: responsePayload}, nil
}

func decodeEnvelopeData(envelope responseEnvelope, target any) error {
	if target == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	return json.Unmarshal(envelope.Data, target)
}

func (s *Service) actionResultFromEnvelope(ctx context.Context, envelope responseEnvelope) (ActionResult, error) {
	result := ActionResult{Code: envelope.Code, Message: envelope.Message}
	if envelope.Code != "need_verify_email" {
		return result, nil
	}
	var data emailVerificationRequired
	if err := decodeEnvelopeData(envelope, &data); err != nil {
		return ActionResult{}, err
	}
	result.Email = data.Email
	result.ExpiresAt = data.ExpiresAt
	return result, s.savePendingEmail(ctx, data.Email)
}

func (s *Service) savePendingEmail(ctx context.Context, email string) error {
	state := s.loadState(ctx)
	if state.UserID != "" {
		return nil
	}
	state.Email = email
	state.EmailVerified = false
	state.SubscriptionStatus = ""
	state.SubscriptionCurrentPeriodEnd = 0
	state.SyncEligible = false
	state.SyncEnabled = false
	state.SessionExpired = false
	return saveAccountState(ctx, state)
}

func normalizeSubscriptionStatus(status string) string {
	trimmed := strings.TrimSpace(status)
	if trimmed == "" {
		return "none"
	}
	return trimmed
}

func loadAccountState(ctx context.Context) (database.AccountState, error) {
	db := database.GetDB()
	if db == nil {
		return database.AccountState{}, fmt.Errorf("database not initialized")
	}
	var state database.AccountState
	err := db.First(&state, accountStateID).Error
	if err == nil {
		return state, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return database.AccountState{}, err
	}
	state = database.AccountState{ID: accountStateID}
	if err := db.Create(&state).Error; err != nil {
		return database.AccountState{}, err
	}
	return state, nil
}

func saveAccountState(ctx context.Context, state database.AccountState) error {
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	state.ID = accountStateID
	state.UpdatedAt = util.GetSystemTimestamp()
	return db.Save(&state).Error
}

func TokenFingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum[:8])
}
