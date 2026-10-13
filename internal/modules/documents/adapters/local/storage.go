package local

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/google/uuid"
	"os"
	storage "panlyackerp/internal/modules/documents/ports"
	"path/filepath"
)

type Storage struct{ Root string }

func (s Storage) Path(key string) (string, error) {
	if _, err := uuid.Parse(key); err != nil {
		return "", errors.New("invalid storage key")
	}
	return filepath.Join(s.Root, key), nil
}
func (s Storage) Save(data []byte, mime string) (storage.File, error) {
	if err := os.MkdirAll(s.Root, 0700); err != nil {
		return storage.File{}, err
	}
	key := uuid.NewString()
	path, err := s.Path(key)
	if err != nil {
		return storage.File{}, err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return storage.File{}, err
	}
	hash := sha256.Sum256(data)
	return storage.File{Key: key, MIME: mime, Hash: hex.EncodeToString(hash[:])}, nil
}
func (s Storage) Remove(key string) error {
	path, err := s.Path(key)
	if err != nil {
		return err
	}
	return os.Remove(path)
}
