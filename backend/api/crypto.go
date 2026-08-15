package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
)

func getEncryptionKey() []byte {
	key := os.Getenv("TOKEN_ENCRYPTION_KEY")
	if key == "" {
		panic("TOKEN_ENCRYPTION_KEY environment variable is missing")
	}
	// Must be exactly 16, 24, or 32 bytes for AES
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		panic("TOKEN_ENCRYPTION_KEY must be exactly 16, 24, or 32 bytes")
	}
	return []byte(key)
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
