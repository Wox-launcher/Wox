package telemetry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"wox/setting"
	"wox/updater"
	"wox/util"
)

const (
	// telemetryEndpoint can be overridden via TELEMETRY_ENDPOINT environment variable
	defaultTelemetryEndpoint = "https://wox-telemetry.qlf.workers.dev/api/v1/telemetry/presence"
	userAgentPrefix          = "Wox/"
	heartbeatIntervalHours   = 24 // hours
)

var telemetryEndpoint = getTelemetryEndpoint()

func getTelemetryEndpoint() string {
	if endpoint := os.Getenv("TELEMETRY_ENDPOINT"); endpoint != "" {
		return endpoint
	}
	return defaultTelemetryEndpoint
}

type PresencePayload struct {
	SchemaVersion int    `json:"schema_version"`
	InstallHash   string `json:"install_hash"`
	OSFamily      string `json:"os_family"`
	WoxVersion    string `json:"wox_version"`
	SentAt        int64  `json:"sent_at"`
}

type PresenceResponse struct {
	Success bool         `json:"success"`
	Message string       `json:"message"`
	Data    PresenceData `json:"data"`
}

type PresenceData struct {
	Accepted   bool  `json:"accepted"`
	ServerTime int64 `json:"server_time"`
}

func SendPresenceIfNeeded(ctx context.Context) {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)

	// Check if anonymous usage stats is enabled
	if !woxSetting.EnableAnonymousUsageStats.Get() {
		util.GetLogger().Debug(ctx, "anonymous usage stats is disabled, skip sending presence")
		return
	}

	state := GetTelemetryState()
	currentVersion := updater.CURRENT_VERSION

	// Check if we should send presence based on throttling rules
	if !state.ShouldSendPresence(currentVersion, heartbeatIntervalHours) {
		util.GetLogger().Debug(ctx, fmt.Sprintf("presence telemetry throttled, last sent at %d", state.LastSentAt))
		return
	}

	// Check OS family - skip if unknown
	osFamily := getOSFamily()
	if osFamily == "unknown" {
		util.GetLogger().Warn(ctx, "unknown OS family, skipping telemetry presence")
		return
	}

	// Send presence in background, don't block
	util.Go(ctx, "telemetry presence", func() {
		sendCtx := util.NewTraceContext()
		err := sendPresence(sendCtx, state, currentVersion, osFamily)
		if err != nil {
			util.GetLogger().Error(sendCtx, fmt.Sprintf("failed to send telemetry presence: %s", err.Error()))
		}
	})
}

func sendPresence(ctx context.Context, state *TelemetryState, version string, osFamily string) error {
	// Calculate install_hash (sha256 of install_id)
	hash := sha256.Sum256([]byte(state.InstallID))
	installHash := hex.EncodeToString(hash[:])

	payload := PresencePayload{
		SchemaVersion: schemaVersion,
		InstallHash:   installHash,
		OSFamily:      osFamily,
		WoxVersion:    version,
		SentAt:        util.GetSystemTimestamp(),
	}

	// Use Wox's existing HTTP utilities to respect proxy settings
	body, err := util.HttpPostWithHeaders(ctx, telemetryEndpoint, payload, map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   userAgentPrefix + updater.CURRENT_VERSION,
	})
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	// Parse response to check for success (body contains JSON response)
	var resp PresenceResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		// Even if we can't parse response, if HttpPost succeeded (no error), update state
		state.UpdateLastSent(version, util.GetSystemTimestamp())
		util.GetLogger().Info(ctx, "telemetry presence sent successfully")
		return nil
	}

	if !resp.Success {
		return fmt.Errorf("server rejected presence: %s", resp.Message)
	}

	// Update state on success
	state.UpdateLastSent(version, util.GetSystemTimestamp())
	util.GetLogger().Info(ctx, "telemetry presence sent successfully")

	return nil
}

func getOSFamily() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "darwin"
	case "linux":
		return "linux"
	default:
		return "unknown"
	}
}

// StartPeriodicHeartbeat starts a background goroutine that sends telemetry
// presence every 24 hours for long-running Wox processes.
func StartPeriodicHeartbeat(ctx context.Context) {
	util.Go(ctx, "telemetry heartbeat", func() {
		ticker := time.NewTicker(time.Duration(heartbeatIntervalHours) * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				SendPresenceIfNeeded(ctx)
			}
		}
	})
}
