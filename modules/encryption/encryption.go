package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
)

// -------------- Globals --------------
var (
	IV_LENGTH  = 16
	KEY_LENGTH = IV_LENGTH * 8
)

// -------------- Functions --------------

// Code from Java
// public byte[] encrypt(Message message) throws GeneralSecurityException {
// 	byte[] initializationVector = new byte[IV_LENGTH];
// 	SecureRandom random = new SecureRandom();
// 	random.nextBytes(initializationVector);
// 	cipher.init(
// 			Cipher.ENCRYPT_MODE,
// 			secretKey,
// 			new GCMParameterSpec(IV_LENGTH * 8, initializationVector));
// 	byte[] encryptedData = cipher.doFinal(message.toByteArray());
// 	byte[] result = new byte[encryptedData.length + IV_LENGTH];
// 	System.arraycopy(encryptedData, 0, result, 0, encryptedData.length);
// 	System.arraycopy(initializationVector, 0, result, encryptedData.length, IV_LENGTH);
// 	return result;
// }

// public Message decrypt(byte[] cypher) throws GeneralSecurityException {
// 	byte[] initializationVector = new byte[IV_LENGTH];
// 	byte[] encryptedData = new byte[cypher.length - IV_LENGTH];
// 	System.arraycopy(cypher, 0, encryptedData, 0, encryptedData.length);
// 	System.arraycopy(cypher, encryptedData.length, initializationVector, 0, IV_LENGTH);
// 	cipher.init(
// 			Cipher.DECRYPT_MODE,
// 			secretKey,
// 			new GCMParameterSpec(IV_LENGTH * 8, initializationVector));
// 	byte[] decryptedData = cipher.doFinal(encryptedData);
// 	return Message.fromByteArray(decryptedData);
// }

// Code for Golang/Go

// EncryptAES encrypts a string using AES, returns the encrypted byte array with the IV added to the end
// Uses AES/GCM/NoPadding
func EncryptAES(input []byte, key string) ([]byte, error) {
	// Create new IV
	initializationVector := make([]byte, IV_LENGTH)
	_, err := rand.Read(initializationVector)
	if err != nil {
		return nil, err
	}

	// Encrypt message
	c, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCMWithNonceSize(c, IV_LENGTH)
	if err != nil {
		return nil, err
	}

	encryptedData := gcm.Seal(nil, initializationVector, input, nil)

	// Add IV to end of encrypted data
	encryptedData = append(encryptedData, initializationVector...)

	return encryptedData, nil
}

// DecryptAES decrypts a byte array using AES, returns the decrypted byte array
// Uses AES/GCM/NoPadding
func DecryptAES(input []byte, key string) ([]byte, error) {
	// Get IV
	encryptedData := make([]byte, len(input)-IV_LENGTH)
	initializationVector := make([]byte, IV_LENGTH)
	copy(encryptedData, input[:len(encryptedData)])
	copy(initializationVector, input[len(encryptedData):])

	// Decrypt message
	c, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCMWithNonceSize(c, IV_LENGTH)
	if err != nil {
		return nil, err
	}

	decryptedData, err := gcm.Open(encryptedData[:0], initializationVector, encryptedData, nil)
	if err != nil {
		return nil, err
	}

	return decryptedData, nil
}
