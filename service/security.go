package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"io"
	"crypto/md5"
)

// CreateHash Generates a sha512 hash any supplied bytes
func CreateHash(content []byte) string {
	hasher := md5.New()
	hasher.Write(content)
	return hex.EncodeToString(hasher.Sum(nil))
}

// Encrypt - Encrypts any arbitrary byte data given a pass phrase
func Encrypt(data []byte, passphrase string) ([]byte, error) {
	passwordHash := CreateHash([]byte(passphrase))
	block, err := aes.NewCipher([]byte(passwordHash))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())

	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// Decrypt - Converts any encrypted data back to its original form given the exact supplied passphrase
func Decrypt(data []byte, passphrase string) ([]byte, error) {
	key := []byte(CreateHash([]byte(passphrase)))
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
