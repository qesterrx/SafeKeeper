// Package config предоставляет функциональность для парсинга и управления
// конфигурацией приложения. Он определяет структуру конфигурации и загружает параметры из аргументов командной строки

package config

import (
	"flag"
	"fmt"
)

// Configuration представляет структуру параметров конфигурации приложения, загружаемых из аргументов командной строки
type Configuration struct {
	// LogFile - путь к файлу для записи логов
	LogFile string
	// LogLevel - уровень детализации логирования. Допустимые значения:DEBUG, INFO, ERROR. По умолчанию установлен "INFO"
	LogLevel string
	// DefDir - директория, содержащая рабочие данные приложения
	DefDir string
	//ClientVersion - Версия клиента
	ClientVersion   int32
	ClientBuildInfo string
}

// ParseParams анализирует аргументы командной строки и возвращает заполненную структуру Configuration
// Функция поддерживает следующие флаги командной строки:
//
//	-f - путь к файлу логирования (по умолчанию пустая строка)
//	-l - уровень логирования: DEBUG, INFO или ERROR (по умолчанию "INFO")
//	-d - директория с данными (по умолчанию пустая строка)
//
// Возвращает:
//   - *Configuration - указатель на структуру с заполненными параметрами
//   - error - ошибка валидации, если уровень логирования имеет
//     некорректное значение
func ParseParams() (*Configuration, error) {
	var cfg Configuration

	flag.StringVar(&cfg.LogFile, "f", "", "Файл логирования")
	flag.StringVar(&cfg.LogLevel, "l", "INFO", "Уровень сообщений логирования [DEBUG/INFO/ERROR]")
	flag.StringVar(&cfg.DefDir, "d", "", "Директория с данными")

	flag.Parse()

	switch cfg.LogLevel {
	case "DEBUG", "INFO", "ERROR":
	default:
		return nil, fmt.Errorf("Некорректное значение уровня сообщений логирования (%s), допустимые [DEBUG/INFO/ERROR]", cfg.LogLevel)
	}

	return &cfg, nil
}
