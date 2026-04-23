package security

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadOrCreateSecretCreatesAndReloadsSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "secret.key")

	secret, err := LoadOrCreateSecret(path)
	if err != nil {
		t.Fatalf("LoadOrCreateSecret(create): %v", err)
	}
	if len(secret) != 32 {
		t.Fatalf("created secret length = %d, want 32", len(secret))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(secret): %v", err)
	}
	if perms := info.Mode().Perm(); perms != 0o600 {
		t.Fatalf("secret perms = %#o, want %#o", perms, 0o600)
	}

	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(secret): %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(string(bytesTrimSpace(encoded)))
	if err != nil {
		t.Fatalf("DecodeString(secret): %v", err)
	}
	if !bytes.Equal(decoded, secret) {
		t.Fatal("stored secret does not match returned secret")
	}

	reloaded, err := LoadOrCreateSecret(path)
	if err != nil {
		t.Fatalf("LoadOrCreateSecret(reload): %v", err)
	}
	if !bytes.Equal(reloaded, secret) {
		t.Fatal("reloaded secret does not match created secret")
	}
}

func TestLoadOrCreateSecretRejectsInvalidSecretFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.key")
	if err := os.WriteFile(path, []byte("%%%not-base64%%%"), 0o600); err != nil {
		t.Fatalf("WriteFile(secret): %v", err)
	}

	_, err := LoadOrCreateSecret(path)
	if err == nil {
		t.Fatal("LoadOrCreateSecret() returned nil error for invalid secret file")
	}
	if !strings.Contains(err.Error(), "decode secret") {
		t.Fatalf("LoadOrCreateSecret() error = %v, want decode secret error", err)
	}
}

func TestEncryptDecryptStringRoundTrip(t *testing.T) {
	secret := bytes.Repeat([]byte{0x42}, 32)

	ciphertext1, err := EncryptString(secret, "top secret")
	if err != nil {
		t.Fatalf("EncryptString(first): %v", err)
	}
	ciphertext2, err := EncryptString(secret, "top secret")
	if err != nil {
		t.Fatalf("EncryptString(second): %v", err)
	}
	if ciphertext1 == ciphertext2 {
		t.Fatal("EncryptString() produced identical ciphertext for repeated input")
	}

	plaintext, err := DecryptString(secret, ciphertext1)
	if err != nil {
		t.Fatalf("DecryptString(): %v", err)
	}
	if plaintext != "top secret" {
		t.Fatalf("DecryptString() = %q, want %q", plaintext, "top secret")
	}
}

func TestEncryptDecryptStringRejectsInvalidKeyLength(t *testing.T) {
	badSecret := []byte("short")

	if _, err := EncryptString(badSecret, "hello"); err == nil {
		t.Fatal("EncryptString() returned nil error for invalid key length")
	}
	if _, err := DecryptString(badSecret, "irrelevant"); err == nil {
		t.Fatal("DecryptString() returned nil error for invalid key length")
	}
}

func TestDecryptStringRejectsInvalidCiphertext(t *testing.T) {
	secret := bytes.Repeat([]byte{0x11}, 32)

	tests := []struct {
		name       string
		ciphertext string
		wantErr    string
	}{
		{
			name:       "invalid base64",
			ciphertext: "%%%not-base64%%%",
			wantErr:    "illegal base64",
		},
		{
			name:       "ciphertext too short",
			ciphertext: base64.StdEncoding.EncodeToString([]byte("tiny")),
			wantErr:    "ciphertext too short",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecryptString(secret, tc.ciphertext)
			if err == nil {
				t.Fatal("DecryptString() returned nil error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("DecryptString() error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}
