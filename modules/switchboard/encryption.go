package switchboard

import (
	"github.com/NeuralNexusDev/neuralnexus-api/modules/encryption"
	"github.com/goccy/go-json"
)

// EncryptMessage encrypts a message
func EncryptMessage(message *Packet, key string) ([]byte, error) {
	// Convert message to JSON
	messageJSON, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}

	// Encrypt message
	encryptedMessage, err := encryption.EncryptAES(messageJSON, key)
	if err != nil {
		return nil, err
	}

	return encryptedMessage, nil
}

// DecryptMessage decrypts a message
func DecryptMessage(encryptedMessage []byte, key string) (*Packet, error) {
	// Decrypt message
	decryptedMessage, err := encryption.DecryptAES(encryptedMessage, key)

	if err != nil {
		return nil, err
	}

	// Take off the padding at the front of the byte array
	// Thanks google common for having nice byte buffers that add padding,
	// In which case I forgot about it and it took me 2 hours to figure out why the message was not decrypting
	// TODO: Convert Java-side to just plain ol byte arrays with no fancy helper classes
	decryptedMessage = decryptedMessage[2:]

	// Convert message from JSON
	var message Packet
	err = json.Unmarshal(decryptedMessage, &message)
	if err != nil {
		return nil, err
	}

	return &message, nil
}
