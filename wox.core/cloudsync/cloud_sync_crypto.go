package cloudsync

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

type CloudSyncKeyProvider interface {
	GetLatestKey(ctx context.Context) ([]byte, int, error)
	GetKey(ctx context.Context, version int) ([]byte, error)
}

type AesGcmCrypto struct {
	keyProvider CloudSyncKeyProvider
}

func NewAesGcmCrypto(keyProvider CloudSyncKeyProvider) *AesGcmCrypto {
	return &AesGcmCrypto{keyProvider: keyProvider}
}

func (c *AesGcmCrypto) Encrypt(ctx context.Context, plaintext string, aad string) (*CloudSyncEncryptedValue, error) {
	if c == nil || c.keyProvider == nil {
		return nil, fmt.Errorf("cloud sync crypto not configured")
	}

	key, version, err := c.keyProvider.GetLatestKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cloud sync key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to read nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), []byte(aad))

	return &CloudSyncEncryptedValue{
		KeyVersion: version,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

func (c *AesGcmCrypto) Decrypt(ctx context.Context, value CloudSyncEncryptedValue, aad string) (string, error) {
	if c == nil || c.keyProvider == nil {
		return "", fmt.Errorf("cloud sync crypto not configured")
	}

	key, err := c.keyProvider.GetKey(ctx, value.KeyVersion)
	if err != nil {
		return "", fmt.Errorf("failed to get cloud sync key version %d: %w", value.KeyVersion, err)
	}

	nonce, err := base64.StdEncoding.DecodeString(value.Nonce)
	if err != nil {
		return "", fmt.Errorf("failed to decode nonce: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(value.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create gcm: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(aad))
	if err != nil {
		return "", fmt.Errorf("failed to decrypt payload: %w", err)
	}

	return string(plaintext), nil
}
