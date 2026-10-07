package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"
)

func getEncryptionKey() []byte {
	key := strings.TrimSpace(os.Getenv("TOKEN_ENCRYPTION_KEY"))
	if key == "" {
		panic("TOKEN_ENCRYPTION_KEY environment variable is missing")
	}

	// Try hex decoding first (e.g. 32, 48, or 64 hex chars -> 16, 24, 32 bytes)
	if keyBytes, err := hex.DecodeString(key); err == nil && (len(keyBytes) == 16 || len(keyBytes) == 24 || len(keyBytes) == 32) {
		return keyBytes
	}

	// Fallback to raw string length (16, 24, or 32 bytes)
	if len(key) == 16 || len(key) == 24 || len(key) == 32 {
		return []byte(key)
	}

	panic("TOKEN_ENCRYPTION_KEY must be exactly 16, 24, or 32 bytes (or 32, 48, 64 hex characters)")
}

// Encrypt encrypts plain text string into base64 encoded string
func Encrypt(text string) (string, error) {
	if text == "" {
		return "", nil
	}
	block, err := aes.NewCipher(getEncryptionKey())
	if err != nil {
		return "", err
	}
	plaintext := []byte(text)
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64 encoded string into plain text string
func Decrypt(cryptoText string) (string, error) {
	if cryptoText == "" {
		return "", nil
	}
	ciphertext, err := base64.URLEncoding.DecodeString(cryptoText)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(getEncryptionKey())
	if err != nil {
		return "", err
	}
	if len(ciphertext) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)
	return string(ciphertext), nil
}
