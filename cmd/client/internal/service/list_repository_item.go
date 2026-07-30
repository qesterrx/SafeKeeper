// Package service предоставляет бизнес-логику и сервисный слой приложения
package service

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/qesterrx/SafeKeeper/cmd/client/internal/model"
	"github.com/qesterrx/SafeKeeper/internal/aes"
)

// ListRepositoryItem представляет отдельное соединение с удаленным репозиторием
// Содержит параметры подключения, учетные данные и зашифрованный AES-токен для безопасного хранения ключа шифрования.
type ListRepositoryItem struct {
	// Name - уникальное имя соединения
	Name string
	// Host - хост сервера
	Host string
	// Port - порт сервера
	Port string
	// Path - локальный путь к директории с данными для этого соединения
	Path string
	// Login - логин для аутентификации на сервере
	Login string
	// Status - текущий статус доступности сервера
	Status string

	// AESToken - зашифрованный AES-токен в формате Base64
	AESToken string
}

// NewListRepositoryItem создает новый экземпляр ListRepositoryItem с указанными параметрами
// Также создает локальную директорию для хранения данных
func NewListRepositoryItem(name, host, port, login, path, aes string) (*ListRepositoryItem, error) {

	lri := ListRepositoryItem{
		Name:  name,
		Host:  host,
		Port:  port,
		Login: login,
		Path:  path,

		AESToken: aes,
	}

	//Создать каталог
	err := os.MkdirAll(lri.Path, 0755)
	if err != nil {
		return nil, fmt.Errorf("ошибка создание каталога данных %w", err)
	}

	return &lri, nil
}

// GetAdress возвращает полный адрес сервера в формате "host:port"
func (lri *ListRepositoryItem) GetAdress() string {
	return lri.Host + ":" + lri.Port
}

// CheckCredentials проверяет учетные данные и расшифровывает AES-токен
// Использует SHA-256 хеш пароля для расшифровки токена и получения ключа
func (lri *ListRepositoryItem) CheckCredentials(password string) ([]byte, error) {

	sha := sha256.Sum256([]byte(password))
	aesToken, err := base64.StdEncoding.DecodeString(lri.AESToken)
	if err != nil {
		return nil, fmt.Errorf("Ошибка расшифровки токена %v", err)
	}

	aesKey, err := aes.DecryptGCM(aesToken, sha[:])
	if err != nil {
		return nil, fmt.Errorf("Ошибка расшифровки токена %v", err)
	}

	return aesKey[:32], nil
}

// GetIfaceRepoItem возвращает интерфейсное представление элемента репозитория в виде модели IfaceRepoItem для использования в других компонентах
func (lri *ListRepositoryItem) GetIfaceRepoItem() *model.IfaceRepoItem {
	return &model.IfaceRepoItem{
		Name:     lri.Name,
		Host:     lri.Host,
		Port:     lri.Port,
		Login:    lri.Login,
		Path:     lri.Path,
		AESToken: lri.AESToken,
	}
}
