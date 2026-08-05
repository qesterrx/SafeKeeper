// Package model определяет структуры данных для работы с объектами SafeKeeper
// Содержит модели для представления пользователей
package model

// DBUser представляет модель пользователя в контексте хранения в базе данных
type DBUser struct {
	ID       int32
	Login    string
	Password string
	AESToken string
}

// AuthUser представляет модель пользователя для авторизации в сервисе
type AuthUser struct {
	Login    string `json:"login"`
	Password string `json:"password"`
	AESToken string `json:"aes_token"`
}
