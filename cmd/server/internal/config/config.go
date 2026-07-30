// Package config предоставляет функциональность для загрузки и парсинга конфигурации приложения
// Поддерживается чтение параметров из командной строки (флагов) и переменных окружения
// Приоритет имеет значение из переменной окружения, которая переопределяет значение флага

package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// NetAddress представляет сетевой адрес, состоящий из хоста и порта
// Реализует интерфейс flag.Value для возможности использования в качестве флага командной строки
type NetAddress struct {
	Host string
	Port int
}

// String возвращает строковое представление сетевого адреса в формате "host:port"
// Реализует интерфейс flag.Value
func (t *NetAddress) String() string {
	return t.Host + ":" + strconv.Itoa(t.Port)
}

// Set парсит строковое значение сетевого адреса и сохраняет его в структуру
// Принимает строку в формате "host:port" возвращает ошибку, если формат не соответствует
// ожидаемому или порт не может быть преобразован в число
// Реализует интерфейс flag.Value
func (t *NetAddress) Set(val string) error {

	p := strings.Split(val, ":")
	if len(p) != 2 {
		return errors.New("params must have format host:port")
	}

	port, err := strconv.Atoi(p[1])
	if err != nil {
		return err
	}

	t.Host = p[0]
	t.Port = port
	return nil

}

// Configuration содержит все настройки, необходимые для работы приложения
// Структура заполняется из флагов командной строки и переменных окружения
type Configuration struct {
	ServerHost  NetAddress // Сетевой адрес, на котором будет запущен HTTP-сервер
	DatabaseDSN string     // Строка подключения к базе данных PostgreSQL (DSN)
	LogLevel    string     // Уровень детализации логирования: DEBUG, INFO или ERROR
	JWTSecret   string     // Секретный ключ для подписи и верификации JWT-токенов
}

// ParseParams загружает конфигурацию приложения из флагов командной строки и переменных окружения
//
// Поддерживаемые флаги и соответствующие переменные окружения:
//   - -a (SKP_SERVER_ADDRESS)  - адрес сервера в формате host:port (по умолчанию localhost:32002)
//   - -d (SKP_DATABASE_DSN)    - строка подключения к PostgreSQL
//   - -l (SKP_LOG_LEVEL)       - уровень логирования (по умолчанию INFO)
//   - -j (SKP_JWT_SECRET)      - секретный ключ для JWT
//
// Возвращает:
//   - *Configuration: указатель на заполненную структуру конфигурации при успешной загрузке
//   - error: ошибка валидации, если обязательные параметры (DatabaseDSN, JWTSecret) не заданы
//     или уровень логирования имеет некорректное значение
func ParseParams() (*Configuration, error) {
	var cfg Configuration

	cfg.ServerHost = NetAddress{Host: "localhost", Port: 32002}
	flag.Var(&cfg.ServerHost, "a", "Адрес сервера в формате host:port")

	flag.StringVar(&cfg.DatabaseDSN, "d", "", "Строка соединения с postgresql")
	flag.StringVar(&cfg.LogLevel, "l", "INFO", "Уровень сообщений логирования [DEBUG/INFO/ERROR]")
	flag.StringVar(&cfg.JWTSecret, "j", "", "Ключ шифрования для JWT")

	flag.Parse()

	if envServerHost := os.Getenv("SKP_SERVER_ADDRESS"); envServerHost != "" {
		cfg.ServerHost.Set(envServerHost)
	}

	if envDatabaseDSN := os.Getenv("SKP_DATABASE_DSN"); envDatabaseDSN != "" {
		cfg.DatabaseDSN = envDatabaseDSN
	}

	if envLogLevel := os.Getenv("SKP_LOG_LEVEL"); envLogLevel != "" {
		cfg.LogLevel = envLogLevel
	}

	if envJWTSecret := os.Getenv("SKP_JWT_SECRET"); envJWTSecret != "" {
		cfg.JWTSecret = envJWTSecret
	}

	if cfg.DatabaseDSN == "" {
		return nil, fmt.Errorf("Необходимо заполнить строку подключения к БД")
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("Необходимо заполнить ключ шифрования для JWT")
	}

	switch cfg.LogLevel {
	case "DEBUG", "INFO", "ERROR":
	default:
		return nil, fmt.Errorf("Некорректное значение уровня сообщений логирования (%s), допустимые [DEBUG/INFO/ERROR]", cfg.LogLevel)
	}

	return &cfg, nil
}
