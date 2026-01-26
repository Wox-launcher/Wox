package cloudsync

import (
	"context"
	"errors"

	"github.com/zalando/go-keyring"
)

var ErrKeyNotFound = errors.New("cloud sync key not found")

type KeyringStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string) error
	Delete(ctx context.Context, key string) error
}

type OSKeyringStore struct {
	service string
}

func NewOSKeyringStore(service string) *OSKeyringStore {
	return &OSKeyringStore{service: service}
}

func (s *OSKeyringStore) Get(ctx context.Context, key string) (string, error) {
	_ = ctx
	value, err := keyring.Get(s.service, key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrKeyNotFound
		}
		return "", err
	}
	return value, nil
}

func (s *OSKeyringStore) Set(ctx context.Context, key string, value string) error {
	_ = ctx
	return keyring.Set(s.service, key, value)
}

func (s *OSKeyringStore) Delete(ctx context.Context, key string) error {
	_ = ctx
	if err := keyring.Delete(s.service, key); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}
		return err
	}
	return nil
}
