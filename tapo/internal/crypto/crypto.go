// Package crypto provides AES-CBC encryption with PKCS7 padding and
// KLAP authentication helpers used by Tapo device transports.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
)

// Encrypt performs AES-CBC encryption with PKCS7 padding.
func Encrypt(key, iv, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: %w", err)
	}

	if len(iv) != block.BlockSize() {
		return nil, fmt.Errorf("crypto: invalid iv size %d", len(iv))
	}

	padded := pkcs7Pad(plaintext, block.BlockSize())
	ciphertext := make([]byte, len(padded))

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	return ciphertext, nil
}

// Decrypt performs AES-CBC decryption and removes PKCS7 padding.
func Decrypt(key, iv, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: %w", err)
	}

	if len(iv) != block.BlockSize() {
		return nil, fmt.Errorf("crypto: invalid iv size %d", len(iv))
	}

	if len(ciphertext) == 0 {
		return nil, errors.New("crypto: ciphertext is empty")
	}

	if len(ciphertext)%block.BlockSize() != 0 {
		return nil, errors.New("crypto: ciphertext is not a multiple of the block size")
	}

	plaintext := make([]byte, len(ciphertext))

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	return pkcs7Unpad(plaintext, block.BlockSize())
}

// pkcs7Pad pads data to a multiple of blockSize using PKCS7.
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padded := make([]byte, len(data)+padding)
	copy(padded, data)
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(padding)
	}
	return padded
}

// pkcs7Unpad removes PKCS7 padding and validates it.
func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("crypto: cannot unpad empty data")
	}

	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize {
		return nil, errors.New("crypto: invalid pkcs7 padding")
	}

	if padding > len(data) {
		return nil, errors.New("crypto: invalid pkcs7 padding")
	}

	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, errors.New("crypto: invalid pkcs7 padding")
		}
	}

	return data[:len(data)-padding], nil
}
