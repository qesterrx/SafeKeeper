// Package model определяет основные типы данных, используемые в приложении
package model

import (
	"time"
)

// MetaObject представляет метаданные объекта хранения. В данной стрйктуре сохраняется файл meta_
type MetaObject struct {
	// Id - уникальный идентификатор объекта в системе
	Id int32 `json:"id"`
	// Name - человекочитаемое имя объекта
	Name string `json:"name"`
	// Kind - тип объекта (определяется константами DataKind: CARD, PASSWORD, TEXT, BIN)
	Kind string `json:"kind"`
	// Version - версия объекта для отслеживания изменений и обеспечения согласованности
	Version int32 `json:"version"`
	// Updated - временная метка последнего обновления объекта
	Updated time.Time `json:"updated"`
	// Status - текущий статус объекта (например, "active", "archived", "deleted")
	Status string `json:"status"`
	// CheckSum - контрольная сумма (хеш) данных объекта для проверки целостности
	CheckSum string `json:"checksum"`
}
