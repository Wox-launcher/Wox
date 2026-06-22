# Task Plan: Settings Cloud Sync Plan

## Goal
Deliver an executable implementation plan for cloud sync of settings, using `wox.core/setting/store.go` as the primary integration point.

## Current Phase
Phase 3

## Phases

### Phase 1: Requirements & Discovery
- [x] Confirm scope of settings to sync (all vs subset, sensitive fields)
- [x] Identify existing settings storage flow in `wox.core/setting/store.go`
- [x] Locate related APIs, UI surfaces, and settings update flows
- [x] Capture constraints (auth, storage backend, offline use, conflicts)
- [x] Document findings in findings.md
- **Status:** complete

### Phase 2: Planning & Structure
- [x] Define sync model (push after save, pull on startup/manual)
- [x] Define batching/debounce/backoff strategy for auto sync
- [x] Define data model and versioning for settings sync payloads (include plugin settings)
- [x] Define per-plugin sync exclusion list (user-configurable, synced)
- [x] Design conflict resolution strategy and merge rules (LWW)
- [x] Plan auth/token storage and security (login, payload encryption, secrets handling)
- [x] Design key bootstrap/recovery for new devices
- [x] Define recovery-code and reset-data policy (loss handling)
- [x] Decide integration points in `wox.core/setting/store.go`
- [x] Identify required API endpoints and WebSocket events
- [x] Identify UI/UX changes and i18n keys (all 4 locales)
- [x] Document decisions with rationale
- **Status:** complete

### Phase 3: Implementation
- [x] Add `CloudSyncDisabledPlugins` to WoxSetting + DTO + Flutter entity (synced config)
- [x] Add local-only CloudSyncState storage (cursor/last_sync/last_error/backoff) in DB or local file
- [x] Write Oplog entries in `WoxSettingStore.Set/Delete` and `PluginSettingStore.Set/Delete/DeleteAll`
- [x] Build CloudSyncManager (queue, debounce, batch, backoff, periodic pull)
- [x] Implement crypto + key management (AES-256-GCM, Argon2id, OS keychain)
- [x] Implement cloud sync client for `/v1/sync/*` endpoints (push/pull/key/reset)
- [x] Apply remote records to local DB with logging bypass + LWW + plugin exclusion filter
- [x] Trigger side effects after apply (`PostSettingUpdate`, plugin setting callbacks)
- [ ] Handle first-time sync bootstrap (cloud wins if snapshot exists)
- [ ] Handle platform-specific settings (sync full PlatformValue JSON)
- [ ] Enforce plugin setting size limits (per-setting/per-plugin caps)
- [x] Respect `SettingValue.syncable` flag (default true) to decide sync eligibility
- [x] Add HTTP endpoints for UI: sync status, push/pull, recovery code, reset flow
- [x] Update Flutter UI: cloud sync card + plugin exclusion list
- [x] Add i18n keys for new UI (4 locales)
- **Status:** in_progress

### Phase 4: Testing & Verification
- [ ] Add/extend Go tests around store sync behavior
- [ ] Validate sync flow across restart/offline scenarios
- [ ] Verify UI states and error handling
- [ ] Document test results in progress.md
- **Status:** pending

### Phase 5: Delivery
- [ ] Review changes for consistency and i18n coverage
- [ ] Provide documentation/notes for configuration
- [ ] Deliver plan and implementation notes to user
- **Status:** pending

## Key Questions
1. Which cloud backend or protocol should be used? (Official paid Wox service, not implemented yet)
2. How should auth be handled? (User login, token-based)
3. What is the conflict resolution policy? (Automatic LWW)
4. Should sync be automatic, manual, or hybrid with scheduling? (Auto after save, pull on startup/manual)
5. What encryption mechanism should be used for synced data? (AES-256-GCM, per-user key)
6. What settings are excluded or treated as sensitive? (All SettingValue synced)
7. Should the per-user key be server-escrowed or require a recovery key on new devices? (Recovery code required)
8. If recovery code is lost, should we reset cloud data automatically or require explicit user action? (Explicit confirmation required)
9. How should remote pull be triggered (periodic polling vs server signal)? (Periodic + startup/manual)
10. First-time sync when both local and cloud have data: which source wins? (Cloud wins)
11. How to handle platform-specific settings? (Sync full PlatformValue JSON)
12. What plugin setting size limits should be enforced? (Propose defaults)

## Decisions Made
| Decision | Rationale |
|----------|-----------|
| Include plugin settings in sync scope | User requirement |
| Official paid Wox cloud service as backend | Product direction |
| Auth via user login (token-based) | Aligns with account system |
| Conflict resolution: automatic LWW | User requirement |
| Auto sync triggers after save | User requirement |
| Encrypt sync payloads with AES-256-GCM (client-side) | Authenticated encryption for synced data |
| Persist per-user encryption key after first login in OS keychain | Enables automatic sync without re-auth each time |
| Recovery code loss requires explicit user confirmation before reset | User requirement |
| New devices require recovery code to unwrap DEK | No server escrow; user-controlled access |
| All SettingValue are syncable | User requirement |
| Per-plugin sync exclusion list stored as a setting | User requirement |
| Remote pull via periodic polling + startup/manual triggers | Practical without server push |
| Recovery code KDF uses Argon2id | User choice |
| Argon2id defaults: mem=64MiB, iter=3, parallelism=2, hash_len=32, salt_len=16 | Security/perf balance for desktop |
| First-time sync uses cloud as source of truth | User requirement |
| Platform-specific settings sync full PlatformValue JSON | User requirement |
| Sync eligibility uses SettingValue.syncable (default true) | User requirement |

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| session-catchup.py failed via `python` (exit code 1) | 1 | Re-ran with `python3` successfully |
| UpdateCloudSyncState return mismatch in CloudSyncManager | 1 | Adjusted call sites to handle returned state + error |

## Notes
- Update phase status as you progress: pending -> in_progress -> complete
- Re-read this plan before major decisions
- Log ALL errors - they help avoid repetition
- CloudSyncManager scaffold is in place but not wired into app startup; client/device/key providers still needed.
- Sync core moved to `wox.core/cloudsync`; setting-specific adapters live in `wox.core/cloudsync/settingadapter` (not yet wired).
