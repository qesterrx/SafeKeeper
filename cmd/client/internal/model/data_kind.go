// Package model определяет основные типы данных, используемые в приложении

package model

// DataKind представляет тип данных, которые клиент может расшифровывать и обрабатывать. Определяет категорию защищенной информации.
type DataKind string

// Константы DataKind определяют поддерживаемые типы данных:
//   - DataKindCard - данные банковской карты
//   - DataKindPswd - пароль или учетные данные
//   - DataKindText - текстовые данные
//   - DataKindBin - бинарные данные
const (
	DataKindCard DataKind = "CARD"
	DataKindPswd DataKind = "PASSWORD"
	DataKindText DataKind = "TEXT"
	DataKindBin  DataKind = "BIN"
)

// IsValid проверяет, является ли значение DataKind допустимым
// Возвращает true, если DataKind соответствует одной из предопределенных констант и false в противном случае
func (dk DataKind) IsValid() bool {
	switch dk {
	case DataKindCard, DataKindPswd, DataKindText, DataKindBin:
		return true
	}
	return false
}

// GetListDataKind возвращает строковое представление всех допустимых типов данных
func GetListDataKind() []string {
	return []string{string(DataKindCard), string(DataKindPswd), string(DataKindText), string(DataKindBin)}
}
