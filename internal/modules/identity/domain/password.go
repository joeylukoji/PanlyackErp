package domain

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"

	"golang.org/x/crypto/argon2"
)

const passwordPrefix = "argon2id-v1"

// Password parameters are fixed until a deliberate migration changes this format.
const (
	passwordPasses    = 3
	passwordMemory    = 64 * 1024
	passwordThreads   = 4
	passwordKeyLength = 32
)

func HashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 128 {
		return "", errors.New("password must contain 12 to 128 bytes")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, passwordPasses, passwordMemory, passwordThreads, passwordKeyLength)
	return passwordPrefix + "$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}

func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 3 || parts[0] != passwordPrefix || len(password) > 128 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil || len(salt) != 16 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(expected) != passwordKeyLength {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, passwordPasses, passwordMemory, passwordThreads, passwordKeyLength)
	return subtle.ConstantTimeCompare(expected, actual) == 1
}
