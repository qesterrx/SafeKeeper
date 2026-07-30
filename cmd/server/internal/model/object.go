// Package model определяет структуры данных для работы с объектами SafeKeeper
// Содержит модели для представления защищённых объектов (данных)

package model

import "time"

// SafeObject представляет защищённый объект в контексте бизнес-логики приложения
// Используется для передачи данных между сервисами и клиентами через API
// Все поля доступны для сериализации в JSON
type SafeObject struct {
	Id       int32     `json:"id"`
	Name     string    `json:"name"`
	Kind     string    `json:"kind"`
	Data     []byte    `json:"data"`
	CheckSum string    `json:"checksum"`
	Updated  time.Time `json:"updated"`
	Version  int32     `json:"version"`
	Client   int32     `json:"client"`
	Deleted  bool      `json:"deleted"`
}

// TranslateToDBObject преобразует SafeObject в DBObject для сохранения в базе данных
// Добавляет идентификатор пользователя-владельца, который передаётся клиентом в контексте
func (so *SafeObject) TranslateToDBObject(user int32) *DBObject {
	return &DBObject{
		Id:       so.Id,
		User:     user,
		Name:     so.Name,
		Kind:     so.Kind,
		Data:     so.Data,
		CheckSum: so.CheckSum,
		Updated:  so.Updated,
		Version:  so.Version,
		Client:   so.Client,
		Deleted:  so.Deleted,
	}
}

// DBObject представляет защищённый объект в контексте хранения в базе данных
// Содержит служебные поля (User, Deleted)
type DBObject struct {
	Id       int32
	User     int32
	Name     string
	Kind     string
	Data     []byte
	CheckSum string
	Updated  time.Time
	Version  int32
	Client   int32
	Deleted  bool
}

// TranslateToSafeObject преобразует DBObject в SafeObject для отправки клиенту
// Исключает служебное поле User
func (dbo *DBObject) TranslateToSafeObject() *SafeObject {
	return &SafeObject{
		Id:       dbo.Id,
		Name:     dbo.Name,
		Kind:     dbo.Kind,
		Data:     dbo.Data,
		CheckSum: dbo.CheckSum,
		Updated:  dbo.Updated,
		Version:  dbo.Version,
		Client:   dbo.Client,
		Deleted:  dbo.Deleted,
	}
}
