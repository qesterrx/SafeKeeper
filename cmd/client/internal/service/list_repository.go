// Package service предоставляет бизнес-логику и сервисный слой приложения
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/qesterrx/SafeKeeper/cmd/client/internal/config"
	"github.com/qesterrx/SafeKeeper/cmd/client/internal/model"
	"github.com/qesterrx/SafeKeeper/internal/logger"
)

// Server определяет интерфейс для взаимодействия с удаленным сервером
// Предоставляет методы для регистрации, аутентификации и проверки доступности
type Server interface {
	// Register выполняет регистрацию нового пользователя на сервере
	// Принимает логин, пароль и адрес сервера. Возвращает AES-токен
	Register(login, pswd, addr string) (string, error)
	// Login выполняет аутентификацию пользователя на сервере
	// Принимает логин, пароль и адрес сервера. Возвращает JWT-токен и AES-токен
	Login(login, pswd, addr string) (string, string, error)
	// Ping проверяет доступность сервера по указанному адресу.
	Ping(addr string) error
}

// ListRepository управляет списком соединений с удаленными репозиториями
type ListRepository struct {
	fileCfgApp string
	server     Server

	cfg *config.Configuration

	DirData string                         `json:"directory"`
	LRIMap  map[string]*ListRepositoryItem `json:"connections"`
}

// NewListRepository создает новый экземпляр ListRepository с загрузкой конфигурации из файла пользователя
// Если конфигурация отсутствует, создается новая с указанной директорией данных.
// Параметры:
//   - ctx: контекст для управления временем жизни
//   - server: реализация интерфейса Server для взаимодействия с сервером
//   - defdir: директория по умолчанию для хранения данных
//
// Возвращает:
//   - *ListRepository: указатель на созданный репозиторий
//   - error: ошибка при создании директорий, чтении или парсинге конфигурации
func NewListRepository(ctx context.Context, server Server, cfg *config.Configuration) (*ListRepository, error) {

	dirCfg, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("ошибка определения каталога конфигурации %w", err)
	}

	dirCfgApp := filepath.Join(dirCfg, "sk")

	lr := ListRepository{cfg: cfg}

	lr.fileCfgApp = filepath.Join(dirCfgApp, "config.json")

	//Проверяем на существование
	if _, err := os.Stat(lr.fileCfgApp); os.IsNotExist(err) {

		if cfg.DefDir == "" {
			return nil, fmt.Errorf("для продолжения необходимо определить папку для размещения данных")
		}

		err := os.MkdirAll(dirCfgApp, 0755)
		if err != nil {
			return nil, fmt.Errorf("ошибка сохранения конфигурации: %w", err)
		}

		lr.DirData = cfg.DefDir
		lr.LRIMap = map[string]*ListRepositoryItem{}

		err = lr.saveCfg()
		if err != nil {
			return nil, err
		}
	}

	//файл есть
	dataCfg, err := os.ReadFile(lr.fileCfgApp)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия файла конфигурации %w", err)
	}

	err = json.Unmarshal(dataCfg, &lr)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия файла конфигурации %w", err)
	}

	lr.server = server

	return &lr, nil
}

// GetItemsList возвращает отсортированный список всех соединений в виде интерфейсных объектов модели IfaceRepoItem
// Возвращает:
//   - []*model.IfaceRepoItem: срез объектов с информацией о соединениях,
//     отсортированный по имени в алфавитном порядке
//   - error: ошибка
func (lr *ListRepository) GetItemsList() ([]*model.IfaceRepoItem, error) {
	sis := []*model.IfaceRepoItem{}

	for _, si := range lr.LRIMap {
		sis = append(sis, si.GetIfaceRepoItem())
	}

	sort.Slice(sis, func(i, j int) bool {
		return sis[i].Name < sis[j].Name
	})

	return sis, nil
}

// GetItem возвращает информацию о соединении по его имени.
// Параметры:
//   - name: имя соединения для поиска
//
// Возвращает:
//   - *model.IfaceRepoItem: объект с данными соединения
//   - error: ошибка
func (lr *ListRepository) GetItem(name string) (*model.IfaceRepoItem, error) {

	tmpName := strings.ToLower(name)

	conn, exists := lr.LRIMap[tmpName]
	if !exists {
		return nil, fmt.Errorf("соединение %s не описано", tmpName)
	}

	return conn.GetIfaceRepoItem(), nil

}

// AddItem добавляет новое соединение в репозиторий
// При добавлении выполняется регистрация или аутентификация на сервере, создается локальная директория для данных и сохраняется конфигурация
//
// Параметры:
//   - name: уникальное имя соединения (регистронезависимое)
//   - host: хост сервера
//   - port: порт сервера
//   - login: логин для аутентификации
//   - pswd: пароль для аутентификации
//   - register: флаг, указывающий на необходимость регистрации (true) или только аутентификации (false)
//
// Возвращает:
//   - error: ошибка
func (lr *ListRepository) AddItem(name string, host string, port string, login string, pswd string, register bool) error {

	//Базовая проверка
	tmpName := strings.ToLower(name)

	if _, exists := lr.LRIMap[tmpName]; exists {
		return fmt.Errorf("соединение %s уже описано", tmpName)
	}

	tmpAESToken := ""
	tmpPath := filepath.Join(lr.DirData, tmpName)

	//Вызов сервера
	if register {
		aesToken, err := lr.server.Register(login, pswd, host+":"+port)
		if err != nil {
			return err
		}

		tmpAESToken = aesToken

	} else {
		_, aesToken, err := lr.server.Login(login, pswd, host+":"+port)
		if err != nil {
			return err
		}

		tmpAESToken = aesToken
	}

	//Создаем новый элемент
	lri, err := NewListRepositoryItem(tmpName, host, port, login, tmpPath, tmpAESToken)
	if err != nil {
		return err
	}

	//Добавляем в мапу
	lr.LRIMap[lri.Name] = lri

	//Обновляем
	err = lr.saveCfg()
	if err != nil {
		return err
	}

	return nil
}

// DelItem удаляет соединение из репозитория по его имени
// Параметры:
//   - name: имя соединения для удаления (регистронезависимое)
//
// Возвращает:
//   - error: ошибка
func (lr *ListRepository) DelItem(name string) error {
	tmpName := strings.ToLower(name)

	conn, exists := lr.LRIMap[tmpName]
	if !exists {
		return fmt.Errorf("соединение %s не описано", tmpName)
	}

	err := os.RemoveAll(conn.Path)
	if err != nil {
		return fmt.Errorf("ошибка при удалении каталога с данными: %w\n", err)
	}

	delete(lr.LRIMap, tmpName)

	err = lr.saveCfg()
	if err != nil {
		return err
	}

	return nil
}

// Cериализовать и записать на диск
func (lr *ListRepository) saveCfg() error {

	jsonCfg, err := json.Marshal(lr)
	if err != nil {
		return fmt.Errorf("ошибка сохранения конфигурации: %w", err)
	}

	err = os.WriteFile(lr.fileCfgApp, jsonCfg, 0644)
	if err != nil {
		return fmt.Errorf("ошибка сохранения конфигурации: %w", err)
	}

	return nil

}

// BackgroundProcess запускает фоновый процесс периодической проверкидоступности всех серверов в репозитории
// Параметры:
//   - ctx: контекст для управления временем жизни процесса
//   - interval: интервал между проверками доступности
func (lr *ListRepository) BackgroundProcess(ctx context.Context, interval time.Duration) {

	logger.Log.Debug("ListRepository.BackgroundProcess START")
	defer logger.Log.Debug("ListRepository.BackgroundProcess STOP")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Log.Debug("ListRepository.BackgroundProcess ctx.Done()")
			return
		case <-ticker.C:

			for _, si := range lr.LRIMap {
				prevStatus := si.Status
				si.Status = "Доступен"
				err := lr.server.Ping(si.GetAdress())
				if err != nil {
					si.Status = err.Error()
				}

				if prevStatus != si.Status {
					logger.Log.Debug("ListRepository.BackgroundProcess ping status: %s, err:%v", si.Status, err)
				}
			}

		}
	}
}
