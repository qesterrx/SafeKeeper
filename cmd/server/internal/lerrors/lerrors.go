// Package lerrors предоставляет типизированные ошибки для логики приложения SafeKeeper
// Все ошибки реализуют стандартный интерфейс error

package lerrors

import "fmt"

// Status определяет числовой код ошибки
// Коды используются для идентификации типа ошибки на стороне клиента
// Значения перечислены в порядке возрастания, начиная с 0 (успех)
type Status int

const (
	//Общий статус успех
	StOk Status = iota

	//Общая ошибка без детализации
	StInternalError

	//Некорректный пароль
	StUserWrongPassword

	//Попытка создать пользователя с зарегистрированным логином
	StUserAlreadyExists

	//Объект по переданному ИД не найден
	StObjectNotFound

	//Конфликт состояний объекта
	StObjectTooOld
)

// LError представляет структурированную ошибку приложения
// Содержит код (числовой идентификатор типа ошибки) и сообщение для логирования
type LError struct {
	Code    Status
	Message string
	Err     error
}

// Error реализует интерфейс error для LError
// Возвращает строковое представление ошибки в формате "[код] сообщение"
// Используется для логирования и отладки
func (se LError) Error() string {
	return fmt.Sprintf("[%d] %s \n %s", se.Code, se.Message, se.Err.Error())
}

// NewLEInternalError создаёт новую ошибку с кодом StInternalError
func NewLEInternalError(err error, msg string) *LError {
	return &LError{Code: StInternalError, Message: msg, Err: err}
}

// NewLEInternalError создаёт новую ошибку с кодом StUserWrongPassword
func NewLEUserWrongPassword(err error, msg string) *LError {
	return &LError{Code: StUserWrongPassword, Message: msg, Err: err}
}

// NewLEInternalError создаёт новую ошибку с кодом StUserAlreadyExists
func NewLEUserAlreadyExists(err error, msg string) *LError {
	return &LError{Code: StUserAlreadyExists, Message: msg, Err: err}
}

// NewLEInternalError создаёт новую ошибку с кодом StObjectNotFound
func NewLEObjectNotFound(err error, msg string) *LError {
	return &LError{Code: StObjectNotFound, Message: msg, Err: err}
}

// NewLEInternalError создаёт новую ошибку с кодом StObjectTooOld
func NewLEObjectTooOld(err error, msg string) *LError {
	return &LError{Code: StObjectTooOld, Message: msg, Err: err}
}
