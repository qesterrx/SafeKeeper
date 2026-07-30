// Package aes предоставляет криптографические функции для шифрования и дешифрования данных с использованием алгоритма AES (Advanced Encryption Standard).

package aes

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"time"
)

// EncryptGCM шифрует данные с использованием AES-256-GCM (Galois/Counter Mode)
func EncryptGCM(source []byte, key []byte) ([]byte, error) {

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	encoded := gcm.Seal(nonce, nonce, source, nil)

	return encoded, nil
}

// DecryptGCM расшифровывает данные, зашифрованные с помощью EncryptGCM
// Выполняет проверку аутентификационного тега, гарантируя целостность данных
func DecryptGCM(source []byte, key []byte) ([]byte, error) {

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(source) < nonceSize {
		return nil, fmt.Errorf("Сообщение недостаточной длины")
	}

	// Извлекаем nonce и зашифрованные данные
	nonce, encodedMsg := source[:nonceSize], source[nonceSize:]

	// Расшифровываем и проверяем аутентификацию
	decoded, err := gcm.Open(nil, nonce, encodedMsg, nil)
	if err != nil {
		return nil, err
	}

	return decoded, nil
}

// GenAESKey генерирует криптостойкий случайный ключ для AES-256
// Использует системный криптографический генератор случайных чисел
func GenAESKey() ([]byte, error) {
	AESKeyBytes := make([]byte, 32)
	_, err := rand.Read(AESKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("Ошибка получения AES ключа %w", err)
	}

	return AESKeyBytes, nil
}

// EncryptStreamCTR выполняет потоковое шифрование данных в режиме CTR (Counter Mode)
// Подходит для шифрования больших файлов и потоков данных, так как не требует загрузки всего объёма данных в память
func EncryptStreamCTR(key []byte, input io.Reader, output io.Writer, len int64, status *string) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return err
	}

	if _, err := output.Write(iv); err != nil {
		return err
	}

	stream := cipher.NewCTR(block, iv)

	processed := 0
	buf := make([]byte, 32*1024)
	for {
		n, err := input.Read(buf)
		if n > 0 {
			stream.XORKeyStream(buf[:n], buf[:n])
			if _, writeErr := output.Write(buf[:n]); writeErr != nil {
				return writeErr
			}

			processed = processed + n
			*status = fmt.Sprintf("Шифрование %.2f%%", float32(processed)/float32(len)*100)
			time.Sleep(1 * time.Millisecond)

		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	*status = "Шифрование завершено"
	return nil
}

// DecryptStreamCTR выполняет потоковое дешифрование данных в режиме CTR (Counter Mode)
// Является обратной операцией к EncryptStreamCTR
func DecryptStreamCTR(key []byte, input io.Reader, output io.Writer, len int64, status *string) error {

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(input, iv); err != nil {
		return err
	}

	stream := cipher.NewCTR(block, iv)

	processed := 0
	buf := make([]byte, 32*1024)
	for {
		n, err := input.Read(buf)
		if n > 0 {
			stream.XORKeyStream(buf[:n], buf[:n])
			if _, writeErr := output.Write(buf[:n]); writeErr != nil {
				return writeErr
			}

			processed = processed + n
			*status = fmt.Sprintf("Расшифровка %.2f%%", float32(processed)/float32(len)*100)
			time.Sleep(1 * time.Millisecond)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	*status = "Расшифровка завершена"
	return nil
}
