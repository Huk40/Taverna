package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	dummyPasswordHash        = "argon2id$v=19$m=19456,t=2,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	argonMemory       uint32 = 19 * 1024
	argonIterations   uint32 = 2
	argonParallelism  uint8  = 1
	argonKeyLength    uint32 = 32
)

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	return fmt.Sprintf("argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonIterations, argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "argon2id" || parts[1] != "v=19" || parts[2] != fmt.Sprintf("m=%d,t=%d,p=%d", argonMemory, argonIterations, argonParallelism) {
		return false
	}
	salt, saltErr := base64.RawStdEncoding.DecodeString(parts[3])
	expected, hashErr := base64.RawStdEncoding.DecodeString(parts[4])
	if saltErr != nil || hashErr != nil || len(salt) != 16 || len(expected) != int(argonKeyLength) {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
