// Package auth owns authentication and authorization: Argon2id password hashing,
// JWT access/refresh tokens, and the RBAC middleware. Security is enforced here,
// in the backend — never in the clients.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters. These are deliberately modest for a course prototype but
// follow the standard shape (memory in KiB, iterations, parallelism). Encoded
// into each hash so we can verify old hashes even if we tune these later.
const (
	argonTime    = 1         // iterations
	argonMemory  = 64 * 1024 // 64 MiB
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

// ErrMismatchedHash is returned when a password does not match the stored hash.
var ErrMismatchedHash = errors.New("password does not match")

// errInvalidHash is returned when a stored hash string is malformed.
var errInvalidHash = errors.New("invalid argon2id hash format")

// HashPassword returns a self-describing Argon2id encoded string of the form
//
//	$argon2id$v=19$m=65536,t=1,p=4$<b64 salt>$<b64 hash>
//
// which is safe to store directly in users.password_hash.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64Salt, b64Hash), nil
}

// VerifyPassword reports whether password matches the given encoded hash. It
// returns ErrMismatchedHash on a wrong password and errInvalidHash on a
// malformed stored value. The comparison is constant-time.
func VerifyPassword(password, encoded string) error {
	params, salt, hash, err := decodeHash(encoded)
	if err != nil {
		return err
	}

	other := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, uint32(len(hash)))
	if subtle.ConstantTimeCompare(hash, other) == 1 {
		return nil
	}
	return ErrMismatchedHash
}

type argonParams struct {
	memory  uint32
	time    uint32
	threads uint8
}

// decodeHash parses an encoded Argon2id string back into its parameters, salt,
// and hash bytes.
func decodeHash(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	// Expected: ["", "argon2id", "v=19", "m=...,t=...,p=...", salt, hash]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argonParams{}, nil, nil, errInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return argonParams{}, nil, nil, errInvalidHash
	}
	if version != argon2.Version {
		return argonParams{}, nil, nil, errInvalidHash
	}

	var p argonParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return argonParams{}, nil, nil, errInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, errInvalidHash
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argonParams{}, nil, nil, errInvalidHash
	}
	return p, salt, hash, nil
}
