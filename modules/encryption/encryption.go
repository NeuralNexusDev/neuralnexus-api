package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
)

// -------------- Globals --------------
var (
	IV_LENGTH  = 16
	KEY_LENGTH = IV_LENGTH * 8
)

// -------------- Functions --------------

// EncryptAES encrypts a string using AES, returns the encrypted byte array with the IV appended
func EncryptAES(input []byte, key string) ([]byte, error) {
	// Generate a new AES cipher using our 32 byte long key
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}

	// Create a new byte array the size of the IV
	iv := make([]byte, IV_LENGTH)

	// Fill the IV with random data
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	// Create a new AES CBC encrypter
	cfb := cipher.NewCFBEncrypter(block, iv)

	// Encrypt the input
	cfb.XORKeyStream(input, input)

	// Append the IV to the encrypted data
	encryptedData := append(iv, input...)

	return encryptedData, nil
}

// DecryptAES decrypts a byte array using AES, returns the decrypted string
func DecryptAES(input []byte, key string) ([]byte, error) {
	// Generate a new AES cipher using our 32 byte long key
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}

	// Get the IV from the input
	iv := input[:IV_LENGTH]

	// Create a new AES CBC decrypter
	cfb := cipher.NewCFBDecrypter(block, iv)

	// Decrypt the input
	cfb.XORKeyStream(input[IV_LENGTH:], input[IV_LENGTH:])

	return input[IV_LENGTH:], nil
}
