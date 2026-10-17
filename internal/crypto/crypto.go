package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"kairo/internal/config"
)

// KeyEnvVar holds the environment variable that can override the secret key.
// It must contain a hex encoded 32-byte key.
const KeyEnvVar = "KAIRO_SECRET_KEY"

// KeyFileName is the name of the file the generated secret key is stored in.
const KeyFileName = "secret.key"

// keySize is the required length of the AES-256 key in bytes.
const keySize = 32

var (
	keyOnce sync.Once
	key     []byte
	keyErr  error
)

// getKey loads or creates the secret key exactly once.
func getKey() ([]byte, error) {
	keyOnce.Do(func() {
		key, keyErr = loadOrCreateKey()
	})

	return key, keyErr
}

// loadOrCreateKey returns the key from KeyEnvVar or from the key file inside
// the kairo path. When no key exists yet, a random key is generated, persisted
// with 0600 permissions and returned.
func loadOrCreateKey() ([]byte, error) {
	if encoded := os.Getenv(KeyEnvVar); encoded != "" {
		return decodeKey(encoded)
	}

	kairoPath := config.GetKairoPath()

	if kairoPath == "" {
		return nil, errors.New("kairo path not initialized")
	}

	keyPath := filepath.Join(kairoPath, KeyFileName)

	data, err := os.ReadFile(keyPath)

	if err == nil {
		return decodeKey(string(data))
	}

	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read key file: %w", err)
	}

	newKey := make([]byte, keySize)

	if _, err := rand.Read(newKey); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	encoded := hex.EncodeToString(newKey) + "\n"

	if err := os.WriteFile(keyPath, []byte(encoded), 0600); err != nil {
		return nil, fmt.Errorf("write key file: %w", err)
	}

	return newKey, nil
}

// decodeKey decodes a hex encoded key and verifies its length.
func decodeKey(encoded string) ([]byte, error) {
	decoded, err := hex.DecodeString(strings.TrimSpace(encoded))

	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}

	if len(decoded) != keySize {
		return nil, fmt.Errorf("key must be %d bytes", keySize)
	}

	return decoded, nil
}

// cipherGCM creates the AES-256-GCM AEAD used by Encrypt and Decrypt.
func cipherGCM() (cipher.AEAD, error) {
	k, err := getKey()

	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(k)

	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)

	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	return gcm, nil
}

// Encrypt encrypts plaintext with AES-256-GCM and returns a base64 encoded
// string of the random nonce followed by the ciphertext.
func Encrypt(plaintext string) (string, error) {
	gcm, err := cipherGCM()

	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())

	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64 encoded nonce and ciphertext produced by Encrypt.
func Decrypt(encoded string) (string, error) {
	gcm, err := cipherGCM()

	if err != nil {
		return "", err
	}

	data, err := base64.StdEncoding.DecodeString(encoded)

	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)

	if err != nil {
		return "", fmt.Errorf("decrypt ciphertext: %w", err)
	}

	return string(plaintext), nil
}
