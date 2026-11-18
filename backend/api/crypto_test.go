package api

import (
	"os"
	"testing"
)

func TestCrypto(t *testing.T) {
	testCases := []struct {
		name string
		key  string
	}{
		{
			name: "64-char hex key (32 bytes AES-256)",
			key:  "687d025c6428949661817148b79e2364403760c5b35c5491be16ad0f84a435c8",
		},
		{
			name: "32-char hex key (16 bytes AES-128)",
			key:  "687d025c6428949661817148b79e2364",
		},
		{
			name: "32-byte raw string",
			key:  "12345678901234567890123456789012",
		},
		{
			name: "16-byte raw string",
			key:  "1234567890123456",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("TOKEN_ENCRYPTION_KEY", tc.key)

			plainText := "gho_secret_github_token_xyz_123"
			cipherText, err := Encrypt(plainText)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}
			if cipherText == "" {
				t.Fatal("Expected non-empty cipherText")
			}

			decrypted, err := Decrypt(cipherText)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}
			if decrypted != plainText {
				t.Fatalf("Expected decrypted '%s', got '%s'", plainText, decrypted)
			}
		})
	}
}
