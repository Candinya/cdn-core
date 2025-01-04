package handlers

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

func (a *App) aesDecrypt(encryptedData []byte) ([]byte, error) {
	c, err := aes.NewCipher(a.esk)
	if err != nil {
		return nil, fmt.Errorf("could not create new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, fmt.Errorf("could not create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return nil, fmt.Errorf("encrypted data too short")
	}

	nonce, ciphertext := encryptedData[:nonceSize], encryptedData[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt ciphertext: %w", err)
	}

	return plaintext, nil
}
