package cloudsync

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"wox/database"

	"golang.org/x/crypto/argon2"
)

const (
	defaultKeyringService = "wox.cloudsync"
	keyringKeyDEK         = "dek"
)

type CloudSyncKeyStatus struct {
	Available bool `json:"available"`
	Version   int  `json:"version"`
}

type KeyManager struct {
	mu             sync.Mutex
	keyring        KeyringStore
	keyClient      CloudSyncKeyClient
	deviceProvider CloudSyncDeviceProvider
	kdfDefaults    CloudSyncKDF
}

type KeyManagerConfig struct {
	Keyring        KeyringStore
	KeyClient      CloudSyncKeyClient
	DeviceProvider CloudSyncDeviceProvider
	KDFDefaults    CloudSyncKDF
}

func NewKeyManager(config KeyManagerConfig) *KeyManager {
	kdf := config.KDFDefaults
	if kdf.Alg == "" {
		kdf = CloudSyncKDF{
			Alg:         "argon2id",
			Version:     19,
			Iter:        3,
			MemKiB:      65536,
			Parallelism: 2,
			HashLen:     32,
		}
	}

	keyring := config.Keyring
	if keyring == nil {
		keyring = NewOSKeyringStore(defaultKeyringService)
	}

	return &KeyManager{
		keyring:        keyring,
		keyClient:      config.KeyClient,
		deviceProvider: config.DeviceProvider,
		kdfDefaults:    kdf,
	}
}

func (m *KeyManager) GetLatestKey(ctx context.Context) ([]byte, int, error) {
	material, err := m.loadMaterial(ctx)
	if err != nil {
		return nil, 0, err
	}
	dek, err := base64.StdEncoding.DecodeString(material.DEK)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to decode dek: %w", err)
	}
	return dek, material.Version, nil
}

func (m *KeyManager) GetKey(ctx context.Context, version int) ([]byte, error) {
	dek, currentVersion, err := m.GetLatestKey(ctx)
	if err != nil {
		return nil, err
	}
	if version != currentVersion {
		return nil, fmt.Errorf("cloud sync key version %d not found", version)
	}
	return dek, nil
}

func (m *KeyManager) GetStatus(ctx context.Context) CloudSyncKeyStatus {
	material, err := m.loadMaterial(ctx)
	if err != nil {
		return CloudSyncKeyStatus{Available: false}
	}
	return CloudSyncKeyStatus{Available: true, Version: material.Version}
}

func (m *KeyManager) InitWithRecoveryCode(ctx context.Context, recoveryCode string, deviceName string) (*CloudSyncKeyInitResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.keyClient == nil {
		return nil, fmt.Errorf("cloud sync key client not configured")
	}

	deviceID, err := m.resolveDeviceID(ctx)
	if err != nil {
		return nil, err
	}

	if deviceName == "" {
		deviceName = resolveDeviceName()
	}

	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		return nil, fmt.Errorf("failed to generate dek: %w", err)
	}

	kdf := m.kdfDefaults
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	kdf.Salt = base64.StdEncoding.EncodeToString(salt)

	kek, err := deriveKEK(recoveryCode, kdf)
	if err != nil {
		return nil, err
	}

	encryptedDEK, err := encryptWithKey(kek, dek)
	if err != nil {
		return nil, err
	}

	resp, err := m.keyClient.InitKey(ctx, CloudSyncKeyInitRequest{
		DeviceID:     deviceID,
		DeviceName:   deviceName,
		KDF:          kdf,
		EncryptedDEK: encryptedDEK,
		KeyVersion:   1,
	})
	if err != nil {
		return nil, err
	}

	version := resp.KeyVersion
	if version == 0 {
		version = 1
	}
	if err := m.saveMaterial(ctx, keyMaterial{Version: version, DEK: base64.StdEncoding.EncodeToString(dek)}); err != nil {
		return nil, err
	}

	m.markBootstrapped(ctx)
	m.tryStartManager(ctx)
	return resp, nil
}

func (m *KeyManager) FetchWithRecoveryCode(ctx context.Context, recoveryCode string) (*CloudSyncKeyFetchResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.keyClient == nil {
		return nil, fmt.Errorf("cloud sync key client not configured")
	}

	deviceID, err := m.resolveDeviceID(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := m.keyClient.FetchKey(ctx, CloudSyncKeyFetchRequest{DeviceID: deviceID})
	if err != nil {
		return nil, err
	}

	kek, err := deriveKEK(recoveryCode, resp.KDF)
	if err != nil {
		return nil, err
	}

	dek, err := decryptWithKey(kek, resp.EncryptedDEK)
	if err != nil {
		return nil, err
	}

	if err := m.saveMaterial(ctx, keyMaterial{Version: resp.KeyVersion, DEK: base64.StdEncoding.EncodeToString(dek)}); err != nil {
		return nil, err
	}

	m.markBootstrapped(ctx)
	m.tryStartManager(ctx)
	return resp, nil
}

func (m *KeyManager) PrepareReset(ctx context.Context) (*CloudSyncKeyResetPrepareResponse, error) {
	if m.keyClient == nil {
		return nil, fmt.Errorf("cloud sync key client not configured")
	}
	return m.keyClient.PrepareKeyReset(ctx)
}

func (m *KeyManager) Reset(ctx context.Context, resetToken string) (*CloudSyncKeyResetResponse, error) {
	if m.keyClient == nil {
		return nil, fmt.Errorf("cloud sync key client not configured")
	}
	resp, err := m.keyClient.ResetKey(ctx, CloudSyncKeyResetRequest{
		ResetToken: resetToken,
		Confirm:    true,
	})
	if err != nil {
		return nil, err
	}

	_ = m.keyring.Delete(ctx, keyringKeyDEK)
	_, _ = UpdateCloudSyncState(ctx, func(state *database.CloudSyncState) {
		state.Bootstrapped = false
	})
	return resp, nil
}

func (m *KeyManager) resolveDeviceID(ctx context.Context) (string, error) {
	if m.deviceProvider == nil {
		return "", fmt.Errorf("cloud sync device provider not configured")
	}
	return m.deviceProvider.DeviceID(ctx)
}

func (m *KeyManager) loadMaterial(ctx context.Context) (keyMaterial, error) {
	if m.keyring == nil {
		return keyMaterial{}, fmt.Errorf("keyring not configured")
	}
	raw, err := m.keyring.Get(ctx, keyringKeyDEK)
	if err != nil {
		return keyMaterial{}, err
	}
	var material keyMaterial
	if err := json.Unmarshal([]byte(raw), &material); err != nil {
		return keyMaterial{}, fmt.Errorf("failed to decode key material: %w", err)
	}
	return material, nil
}

func (m *KeyManager) saveMaterial(ctx context.Context, material keyMaterial) error {
	if m.keyring == nil {
		return fmt.Errorf("keyring not configured")
	}
	payload, err := json.Marshal(material)
	if err != nil {
		return fmt.Errorf("failed to encode key material: %w", err)
	}
	return m.keyring.Set(ctx, keyringKeyDEK, string(payload))
}

func (m *KeyManager) markBootstrapped(ctx context.Context) {
	_, _ = UpdateCloudSyncState(ctx, func(state *database.CloudSyncState) {
		state.Bootstrapped = true
	})
}

func (m *KeyManager) tryStartManager(ctx context.Context) {
	if service := GetService(); service != nil {
		service.StartManager(ctx)
	}
}

type keyMaterial struct {
	Version int    `json:"version"`
	DEK     string `json:"dek"`
}

func deriveKEK(recoveryCode string, kdf CloudSyncKDF) ([]byte, error) {
	if strings.ToLower(kdf.Alg) != "argon2id" {
		return nil, fmt.Errorf("unsupported kdf: %s", kdf.Alg)
	}
	salt, err := base64.StdEncoding.DecodeString(kdf.Salt)
	if err != nil {
		return nil, fmt.Errorf("invalid kdf salt: %w", err)
	}
	code := NormalizeRecoveryCode(recoveryCode)
	if code == "" {
		return nil, fmt.Errorf("recovery code is empty")
	}
	key := argon2.IDKey([]byte(code), salt, uint32(kdf.Iter), uint32(kdf.MemKiB), uint8(kdf.Parallelism), uint32(kdf.HashLen))
	return key, nil
}

func encryptWithKey(key []byte, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	payload := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(payload), nil
}

func decryptWithKey(key []byte, payload string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encrypted dek: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("encrypted dek payload too short")
	}
	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt dek: %w", err)
	}
	return plaintext, nil
}

func resolveDeviceName() string {
	if name, err := os.Hostname(); err == nil && name != "" {
		return name
	}
	return "wox-device"
}
