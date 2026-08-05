// Package model определяет основные типы данных, используемые в приложении
package model

import (
	"io"
	"time"
)

// TransportObject представляет объект, предназначенный для передачи по сети
// Используется в транспортном слое для обмена данными между клиентом и сервером
type TransportObject struct {
	// Id - уникальный идентификатор объекта в системе
	Id int32
	// Name - человекочитаемое имя объекта
	Name string
	// Kind - тип объекта (определяется константами DataKind: CARD, PASSWORD, TEXT, BIN)
	Kind string
	// Version - версия объекта для отслеживания изменений
	Version int32
	// Updated - временная метка последнего обновления объекта
	Updated time.Time
	// Client - идентификатор клиента, которому принадлежит объект
	Client int32
	// Deleted - флаг, указывающий, помечен ли объект как удаленный
	Deleted bool
	// Data - поток для чтения/записи данных объекта (реализует интерфейс io.ReadWriteCloser)
	Data io.ReadWriteCloser
	// DataSize - размер данных объекта в байтах
	DataSize int32
	// CheckSum - контрольная сумма (хеш) данных для проверки целостности при передаче
	CheckSum string
}
