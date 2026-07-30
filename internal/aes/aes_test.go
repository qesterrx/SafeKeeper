package aes

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptGCM(t *testing.T) {
	key := []byte("12345678901234567890123456789012") // 32 bytes for AES-256
	plaintext := []byte("This is a secret message that needs encryption!")

	encrypted, err := EncryptGCM(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptGCM() error = %v", err)
	}

	if len(encrypted) == 0 {
		t.Error("EncryptGCM() returned empty encrypted data")
	}

	if bytes.Equal(encrypted, plaintext) {
		t.Error("EncryptGCM() did not encrypt data")
	}

	decrypted, err := DecryptGCM(encrypted, key)
	if err != nil {
		t.Fatalf("DecryptGCM() error = %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("DecryptGCM() = %v, want %v", decrypted, plaintext)
	}
}

func TestEncryptGCM_InvalidKey(t *testing.T) {
	key := []byte("short") // Invalid key length
	plaintext := []byte("test")

	_, err := EncryptGCM(plaintext, key)
	if err == nil {
		t.Error("EncryptGCM() expected error for invalid key, got nil")
	}
}

func TestDecryptGCM_InvalidData(t *testing.T) {
	key := []byte("12345678901234567890123456789012")
	invalidData := []byte("too short")

	_, err := DecryptGCM(invalidData, key)
	if err == nil {
		t.Error("DecryptGCM() expected error for invalid data, got nil")
	}
}

func TestDecryptGCM_WrongKey(t *testing.T) {
	key1 := []byte("12345678901234567890123456789012")
	key2 := []byte("12345678901234567890123456789013")
	plaintext := []byte("secret message")

	encrypted, err := EncryptGCM(plaintext, key1)
	if err != nil {
		t.Fatalf("EncryptGCM() error = %v", err)
	}

	_, err = DecryptGCM(encrypted, key2)
	if err == nil {
		t.Error("DecryptGCM() expected error for wrong key, got nil")
	}
}

func TestGenAESKey(t *testing.T) {
	key1, err := GenAESKey()
	if err != nil {
		t.Fatalf("GenAESKey() error = %v", err)
	}

	if len(key1) != 32 {
		t.Errorf("GenAESKey() returned key of length %d, want 32", len(key1))
	}

	key2, err := GenAESKey()
	if err != nil {
		t.Fatalf("GenAESKey() error = %v", err)
	}

	if bytes.Equal(key1, key2) {
		t.Error("GenAESKey() returned same key twice, expected random")
	}
}

func TestEncryptDecryptStreamCTR(t *testing.T) {
	key := []byte("12345678901234567890123456789012")
	plaintext := []byte("This is a test data for stream encryption! " +
		"It should work with large files as well.")

	input := bytes.NewReader(plaintext)
	output := &bytes.Buffer{}
	status := ""

	err := EncryptStreamCTR(key, input, output, int64(len(plaintext)), &status)
	if err != nil {
		t.Fatalf("EncryptStreamCTR() error = %v", err)
	}

	if status != "Шифрование завершено" {
		t.Errorf("Status = %v, want 'Шифрование завершено'", status)
	}

	encrypted := output.Bytes()
	if bytes.Equal(encrypted, plaintext) {
		t.Error("EncryptStreamCTR() did not encrypt data")
	}

	// Decrypt
	decryptInput := bytes.NewReader(encrypted)
	decryptOutput := &bytes.Buffer{}
	status = ""

	err = DecryptStreamCTR(key, decryptInput, decryptOutput, int64(len(encrypted)), &status)
	if err != nil {
		t.Fatalf("DecryptStreamCTR() error = %v", err)
	}

	if status != "Расшифровка завершена" {
		t.Errorf("Status = %v, want 'Расшифровка завершена'", status)
	}

	decrypted := decryptOutput.Bytes()
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypted data = %v, want %v", decrypted, plaintext)
	}
}

func TestEncryptStreamCTR_WrongKey(t *testing.T) {
	key := []byte("12345")
	plaintext := []byte("test data")

	input := bytes.NewReader(plaintext)
	output := &bytes.Buffer{}
	status := ""

	err := EncryptStreamCTR(key, input, output, int64(len(plaintext)), &status)
	if err == nil {
		t.Error("EncryptStreamCTR() expected error for invalid key, got nil")
	}
}

func TestDecryptStreamCTR_WrongIV(t *testing.T) {
	key := []byte("12345678901234567890123456789012")
	plaintext := []byte("test data")

	input := bytes.NewReader(plaintext)
	output := &bytes.Buffer{}
	status := ""

	err := DecryptStreamCTR(key, input, output, int64(len(plaintext)), &status)
	if err == nil {
		t.Error("DecryptStreamCTR() expected error for missing IV, got nil")
	}
}
