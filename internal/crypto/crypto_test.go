package crypto

import (
	"strings"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	t.Setenv(KeyEnvVar, strings.Repeat("ab", keySize))

	plaintext := "gsk_secret_api_key_1234567890"

	encrypted, err := Encrypt(plaintext)

	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if encrypted == plaintext {
		t.Fatal("encrypted value must differ from plaintext")
	}

	if strings.Contains(encrypted, plaintext) {
		t.Fatal("encrypted value must not contain the plaintext")
	}

	decrypted, err := Decrypt(encrypted)

	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Fatalf("decrypt mismatch: got %q want %q", decrypted, plaintext)
	}
}

func TestEncryptRandomNonce(t *testing.T) {
	t.Setenv(KeyEnvVar, strings.Repeat("cd", keySize))

	first, err := Encrypt("same plaintext")

	if err != nil {
		t.Fatalf("encrypt first: %v", err)
	}

	second, err := Encrypt("same plaintext")

	if err != nil {
		t.Fatalf("encrypt second: %v", err)
	}

	if first == second {
		t.Fatal("encrypting the same plaintext twice must produce different ciphertexts")
	}
}

func TestDecryptInvalidInput(t *testing.T) {
	t.Setenv(KeyEnvVar, strings.Repeat("ef", keySize))

	if _, err := Decrypt("not base64 !!!"); err == nil {
		t.Fatal("decrypt must fail for invalid base64")
	}

	if _, err := Decrypt("AAAA"); err == nil {
		t.Fatal("decrypt must fail for short ciphertext")
	}
}
