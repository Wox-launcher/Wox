# Findings & Decisions

## Requirements
- Implement a cloud sync feature for settings.
- Primary integration point should be `wox.core/setting/store.go`.
- Produce an executable plan before implementation.
- Include plugin settings in sync scope.
- Backend is an official paid Wox cloud service (not implemented yet).
- Auth via user login (token-based).
- Conflict handling is automatic last-write-wins (LWW).
- Sync should trigger automatically after save.
- Sync payloads must be encrypted.
- Encryption key should be provisioned at first login and persisted for later sync.
- New device should be able to sync after login (key bootstrap/recovery required).
- Recovery code implies user-held secret; if lost and no existing device, data must be reset.
- Recovery code loss requires explicit user confirmation before cloud data reset.
- Sync eligibility uses `SettingValue.syncable` (default true).
- Users can exclude specific plugins from sync; this exclusion list is itself a synced setting.
- Remote pull should be periodic polling plus startup/manual triggers (no server signal required).
- Server implementation lives in a separate repo; Wox only needs client integration + API contract.
- Auto push batching: debounce 2s, max 100 changes or 1MB payload per push, exponential backoff (1s -> 2m with jitter).
- Recovery code KDF: Argon2id.
- First-time sync needs bootstrap logic: detect cloud snapshot existence and reconcile with local settings.
- First-time sync needs a user choice when both sides have data (cloud wins vs local wins vs two-way).
- First-time sync should use cloud as source of truth when snapshot exists.
- Platform-specific settings will sync the full PlatformValue JSON (win/mac/linux) without special handling.
- Plugin setting size limits are required to avoid abnormal traffic.

## Research Findings
- Settings are persisted in `wox.db` via Gorm models `WoxSetting` (Key, Value) and `PluginSetting` (PluginID, Key, Value).
- `WoxSettingStore` serializes primitives to strings and complex types to JSON; `PlatformValue` stores a JSON struct of per-OS values.
- `SettingValue.Set` writes via `SettingStore.Set`; no sync hooks are currently invoked.
- `SettingValue` contains a `syncable` flag (currently unused), and should be the canonical sync eligibility signal.
<!-- Platform-specific Wox settings are stored as `PlatformValue`; we will sync the full JSON blob without platform filtering. -->
- No existing keychain/credential utility in `wox.core`; need new secure storage for DEK.
- Cloud sync state (cursor/last sync/backoff) should be stored outside `SettingValue` to avoid syncing internal metadata.
- `Oplog` table includes `SyncedToCloud` and `LogOplog` exists in `WoxSettingStore`, but no current callers.
- Core settings API: `/setting/wox` (read) and `/setting/wox/update` (write) in `wox.core/ui/router.go`.
- Plugin settings API: `/setting/plugin/update` uses `APIImpl.SaveSetting` and `PluginSettingStore`.
- `PostSettingUpdate` in `wox.core/ui/manager.go` applies side effects (tray, hotkeys, lang, autostart, MCP server).
- Flutter UI calls `/setting/wox` + `/setting/wox/update` via `WoxApi`; settings are reloaded after update.
- Backup/restore copies the full user data directory; `wox.db` lives under user data.

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| Include plugin settings in sync scope | User requirement |
| Backend is official paid Wox cloud service | Product direction |
| Auth via user login (token-based) | Aligns with account system |
| Conflict resolution uses LWW | User requirement |
| Auto sync triggers after save | User requirement |
| Payload encryption uses AES-256-GCM with per-user key | Authenticated encryption and cross-device decrypt |
| Persist per-user encryption key after first login in OS keychain | Enables automatic sync without re-auth each time |
| All SettingValue are syncable | User requirement |
| Per-plugin sync exclusion list stored as a synced setting | User requirement |
| Remote pull via periodic polling + startup/manual triggers | Simpler without server push |
| Push batching uses debounce + max batch + exponential backoff | Avoids frequent network calls |
| Recovery code KDF uses Argon2id | User choice |
| First-time sync uses cloud snapshot when available | User requirement |
| Platform-specific settings sync full PlatformValue JSON | User requirement |
| Sync eligibility uses SettingValue.syncable (default true) | User requirement |

## Issues Encountered
| Issue | Resolution |
|-------|------------|
| session-catchup.py required `python3` instead of `python` | Retried with `python3` |

## Resources
- `wox.core/setting/store.go`
- `wox.core/setting/value.go`
- `wox.core/setting/wox_setting.go`
- `wox.core/setting/manager.go`
- `wox.core/setting/backup_restore.go`
- `wox.core/ui/router.go`
- `wox.core/ui/manager.go`
- `wox.core/ui/dto/setting_dto.go`
- `wox.core/database/database.go` (WoxSetting, PluginSetting, Oplog)
- `wox.core/plugin/api.go` (SaveSetting)
- `wox.ui.flutter/wox/lib/api/wox_api.dart`
- `wox.ui.flutter/wox/lib/controllers/wox_setting_controller.dart`
- `wox.ui.flutter/wox/lib/entity/wox_setting.dart`

## API Draft (Client <-> Cloud Service)
### Common
- **Base**: `/v1`
- **Auth**: `Authorization: Bearer <access_token>`
- **Headers**: `X-Device-Id`, `X-App-Version`, `X-Platform`, `X-Trace-Id` (optional)
- **Time**: server returns `server_ts` (ms since epoch) and LWW uses server time.

### Encryption Envelope
- **Alg**: `AES-256-GCM`
- **Key**: per-user DEK, stored locally (OS keychain).
- **AAD**: `entity_type + ":" + plugin_id + ":" + key + ":" + op` (UTF-8; plugin_id empty for wox settings).
- **EncryptedValue**:
  - `key_version` (int)
  - `nonce` (base64, 12 bytes)
  - `ciphertext` (base64, includes GCM tag)
- **Plaintext**: serialized setting value string (same as DB storage).

### Key Bootstrap / Recovery
1) **POST** `/sync/key/init`
   - Use on first device after login.
   - Request: `{device_id, device_name, kdf: {alg, salt, iter, mem_kib, parallelism, hash_len, version}, encrypted_dek, key_version}`
   - Response: `{key_version, created_at}`
2) **POST** `/sync/key/fetch`
   - Request: `{device_id}`
   - Response: `{key_version, kdf: {alg, salt, iter, mem_kib, parallelism, hash_len, version}, encrypted_dek}`
3) **POST** `/sync/key/reset/prepare`
   - Response: `{reset_token, expires_at}` (explicit confirmation flow)
4) **POST** `/sync/key/reset`
   - Request: `{reset_token, confirm: true}`
   - Response: `{reset_at}`
   - Behavior: wipe cloud data + key; user must re-bootstrap key.

### Sync Push/Pull
- **Record identity**: `(entity_type, plugin_id, key)` unique.
- **Operations**: `upsert` | `delete`

1) **POST** `/sync/push`
   - Request:
     ```
     {
       "device_id": "...",
       "changes": [
         {
           "change_id": "uuid",
           "entity_type": "wox_setting|plugin_setting",
           "plugin_id": "optional",
           "key": "setting_key",
           "op": "upsert|delete",
           "client_ts": 1234567890,
           "value": { "key_version": 1, "nonce": "...", "ciphertext": "..." }
         }
       ]
     }
     ```
   - Response:
     ```
     {
       "server_ts": 1234567999,
       "applied": [{"change_id":"...","status":"ok|ignored","server_ts":123}],
       "next_cursor": "..."
     }
     ```
   - Server applies LWW by `server_ts`.

2) **POST** `/sync/pull`
   - Request: `{device_id, cursor, limit}`
   - Response:
     ```
     {
       "records": [
         {
           "entity_type": "wox_setting|plugin_setting",
           "plugin_id": "optional",
           "key": "setting_key",
           "op": "upsert|delete",
           "server_ts": 1234567999,
           "client_ts": 1234567000,
           "value": { "key_version": 1, "nonce": "...", "ciphertext": "..." }
         }
       ],
       "next_cursor": "...",
       "has_more": false
     }
     ```

3) **POST** `/sync/snapshot`
   - Request: `{device_id}`
   - Response: same as `/sync/pull` with full dataset (paged).

### Device Metadata (optional)
1) **POST** `/devices/register`
   - Request: `{device_id, device_name, platform}`
   - Response: `{ok: true}`

### KDF Defaults (Recovery Code)
- `alg`: `argon2id`
- `version`: `19`
- `mem_kib`: `65536` (64 MiB)
- `iter`: `3`
- `parallelism`: `2`
- `hash_len`: `32`
- `salt_len`: `16` bytes (server generates, base64)

## Visual/Browser Findings
- None
