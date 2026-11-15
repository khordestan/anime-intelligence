// Package password implements bounded Argon2id PHC hashing using x/crypto.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const memory = 64 * 1024 // KiB; budget concurrent authentication attempts separately.
const iterations = 3
const parallelism = 2
const MaxBytes = 1024

var ErrInvalid = errors.New("password must contain 12 to 1024 bytes")

func Hash(value string) (string, error) {
	if len(value) < 12 || len(value) > MaxBytes {
		return "", ErrInvalid
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(value), salt, iterations, memory, parallelism, 32)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", memory, iterations, parallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify only accepts the fixed resource profile issued by this service. This
// prevents malformed stored hashes from causing unbounded CPU or allocation.
func Verify(encoded, value string) bool {
	if len(value) > MaxBytes || len(encoded) > 512 {
		return false
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=65536,t=3,p=2" {
		return false
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false
	}
	expected, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(expected) != 32 {
		return false
	}
	actual := argon2.IDKey([]byte(value), salt, iterations, memory, parallelism, 32)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
