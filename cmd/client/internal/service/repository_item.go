// Package service предоставляет бизнес-логику и сервисный слой приложения
package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/qesterrx/SafeKeeper/cmd/client/internal/model"
	"github.com/qesterrx/SafeKeeper/internal/aes"
	"github.com/qesterrx/SafeKeeper/internal/logger"
)

// itemStatus представляет статус объекта в процессе его жизненного цикла
type itemStatus string

// Константы статусов элемента репозитория определяют все возможные состояния объекта от создания до синхронизации с сервером
const (
	itemStatusEmpty          itemStatus = "EMPTY"          // Пустой начальный статус
	itemStatusNew            itemStatus = "NEW"            // Новый объект
	itemStatusSynced         itemStatus = "SYNCED"         // Синхронизирован с сервером
	itemStatusConflict       itemStatus = "CONFLICT"       // Конфликт версий
	itemStatusError          itemStatus = "ERROR"          // Ошибка
	itemStatusCreating       itemStatus = "CREATING"       // Создается
	itemStatusCreated        itemStatus = "CREATED"        // Создан локально, готов к отправке
	itemStatusSendingCreated itemStatus = "SENDINGCREATED" // Отправляется на сервер (создание)
	itemStatusEditing        itemStatus = "EDITING"        // Редактируется
	itemStatusEdited         itemStatus = "EDITED"         // Отредактирован, готов к отправке
	itemStatusSendingEdited  itemStatus = "SENDINGEDITED"  // Отправляется на сервер (редактирование)
	itemStatusDeleted        itemStatus = "DELETED"        // Помечен на удаление
	itemStatusSendingDeleted itemStatus = "SENDINGDELETED" // Отправляется на сервер (удаление)
	itemStatusReceaving      itemStatus = "RECEAVING"      // Получается с сервера
	itemStatusUpdating       itemStatus = "UPDATING"       // Обновляется с сервера
)

// IsValid проверяет, является ли статус допустимым
func (is itemStatus) IsValid() bool {
	switch is {
	case
		itemStatusEmpty,
		itemStatusNew,
		itemStatusSynced,
		itemStatusConflict,
		itemStatusError,
		itemStatusCreating,
		itemStatusCreated,
		itemStatusSendingCreated,
		itemStatusEditing,
		itemStatusEdited,
		itemStatusSendingEdited,
		itemStatusDeleted,
		itemStatusSendingDeleted,
		itemStatusReceaving,
		itemStatusUpdating:
		return true
	}
	return false
}

// repositoryItem представляет элемент репозитория с метаданными, статусами и файловыми дескрипторами для хранения зашифрованных данных
type repositoryItem struct {
	id      int32
	name    string
	kind    model.DataKind
	version int32
	client  int32
	updated time.Time
	message string

	status     itemStatus
	statusPrev itemStatus

	metafile string
	datafile string
	tmpfile  string

	mu sync.Mutex
}

// newRepositoryItem создает новый элемент репозитория с указанными параметрами
// Создает файлы метаданных и данных в указанной директории
//
// Параметры:
//   - id: идентификатор объекта
//   - name: имя объекта
//   - kind: тип объекта (CARD, PASSWORD, TEXT, BIN)
//   - path: путь к директории хранения
//
// Возвращает:
//   - *repositoryItem: указатель на созданный элемент
//   - error: ошибка
func newRepositoryItem(id int32, name string, kind string, path string) (*repositoryItem, error) {

	dataKind := model.DataKind(kind)
	if !dataKind.IsValid() {
		return nil, fmt.Errorf("Тип объекта не поддерживается сервисным слоем")
	}

	t := time.Now()
	hash := sha256.Sum256([]byte(name + t.Format("2006-01-02 15:04:05")))
	filename := base64.RawURLEncoding.EncodeToString(hash[:])

	ri := repositoryItem{
		id:         id,
		name:       name,
		kind:       dataKind,
		version:    0,
		status:     itemStatusNew,
		statusPrev: itemStatusEmpty,
		metafile:   filepath.Join(path, "meta_"+filename),
		datafile:   filepath.Join(path, "data_"+filename),
		tmpfile:    filepath.Join(path, "tmp_"+filename),
	}

	err := ri.saveMeta()
	if err != nil {
		return nil, err
	}

	file, err := os.Create(ri.datafile)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать файл данных %s:%w", ri.datafile, err)
	}
	file.Close()

	return &ri, nil
}

// restoreRepositoryItemFromMeta восстанавливает элемент репозитория из сохраненных метаданных. Проверяет валидность типа и статуса
//
// Параметры:
//   - path: путь к директории
//   - filename: имя файла метаданных
//   - meta: объект метаданных
//
// Возвращает:
//   - *repositoryItem: восстановленный элемент или nil, если объект был удален.
func restoreRepositoryItemFromMeta(path, filename string, meta *model.MetaObject) *repositoryItem {

	ri := repositoryItem{
		id:         meta.Id,
		name:       meta.Name,
		kind:       model.DataKind(meta.Kind),
		version:    meta.Version,
		updated:    meta.Updated,
		status:     itemStatus(meta.Status),
		statusPrev: itemStatusEmpty,
		metafile:   filepath.Join(path, filename),
		datafile:   filepath.Join(path, strings.Replace(filename, "meta_", "data_", 1)),
		tmpfile:    filepath.Join(path, strings.Replace(filename, "meta_", "tmp_", 1)),
	}

	if !ri.kind.IsValid() {
		ri.message = "Тип объекта не поддерживается сервисным слоем"
		ri.status = itemStatusError
		return &ri
	}

	if !ri.status.IsValid() {
		ri.message = fmt.Sprintf("Сатус объекта сохраненный в метаданных [%s] не поддерживается сервисным слоем", string(ri.status))
		ri.status = itemStatusError
		return &ri
	}

	//Восстановим статус
	ri.RestorePrevStatus()

	//Посмотрим что можно сделать по текущему статусу
	if ri.status == itemStatusNew {
		//Если восстановление идет из new значит ничего не успели сделать кроме как создать пустышки - удаляем
		err := ri.Delete()
		if err != nil {
			ri.status = itemStatusError
			ri.message = err.Error()
		} else {
			//Пустой результат говорит о том что объекта нет
			return nil
		}
	}

	return &ri
}

// saveMeta сохраняет метаданные элемента в файл в формате JSON
func (ri *repositoryItem) saveMeta() error {

	if ri.status == itemStatusError {
		return fmt.Errorf("объект в этом статусе не может быть сохранен")
	}

	mo := model.MetaObject{
		Id:      ri.id,
		Name:    ri.name,
		Kind:    string(ri.kind),
		Version: ri.version,
		Updated: ri.updated,
		Status:  string(ri.status),
	}

	jsonMo, err := json.Marshal(mo)
	if err != nil {
		return fmt.Errorf("не удалось создать файл метаданных %s:%w", ri.metafile, err)
	}

	err = os.WriteFile(ri.metafile, jsonMo, 0644)
	if err != nil {
		return fmt.Errorf("не удалось создать файл метаданных %s:%w", ri.metafile, err)
	}

	return nil
}

// WriteEncryptData шифрует данные из источника и записывает их в файл данных
// Использует AES-CTR для потокового шифрования
// Параметры:
//   - src: источник данных для чтения
//   - len: размер данных в байтах
//   - key: AES-ключ для шифрования
//
// Возвращает:
//   - error: ошибка
func (ri *repositoryItem) WriteEncryptData(src io.Reader, len int64, key []byte) error {

	dst, err := os.OpenFile(ri.tmpfile, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл данных для записи: %w", err)
	}

	err = aes.EncryptStreamCTR(key, src, dst, len, &ri.message)
	if err != nil {
		dst.Close()
		return fmt.Errorf("ошибка шифрования данных: %w", err)
	}

	// Если не смогли закрыть файл то пытаемся его удалить и вернуть ошибку
	err = dst.Close()
	if err != nil {
		os.Remove(ri.tmpfile)
		return fmt.Errorf("ошибка шифрования данных: %w", err)
	}

	//Вот тут идет переименование с заменой, не факт что это поддерживается всеми ОС
	err = os.Rename(ri.tmpfile, ri.datafile)
	if err != nil {
		return fmt.Errorf("ошибка шифрования данных: %w", err)
	}

	return nil

}

// ReadEncryptData читает и расшифровывает данные из файла данных
// Использует AES-CTR для потокового расшифрования
//
// Параметры:
//   - dst: приемник для записи расшифрованных данных
//   - key: AES-ключ для расшифрования
//
// Возвращает:
//   - error: ошибка
func (ri *repositoryItem) ReadEncryptData(dst io.Writer, key []byte) error {

	src, err := os.Open(ri.datafile)
	if err != nil {
		return fmt.Errorf("ошибка чтения данных: %w", err)
	}
	defer src.Close()

	fileInfo, err := src.Stat()
	if err != nil {
		return fmt.Errorf("ошибка чтения данных: %w", err)
	}

	err = aes.DecryptStreamCTR(key, src, dst, fileInfo.Size(), &ri.message)
	if err != nil {
		return fmt.Errorf("ошибка расшифровки данных: %w", err)
	}

	return nil

}

// Delete удаляет все файлы элемента (метаданные, данные и временный файл)
func (ri *repositoryItem) Delete() error {
	err := os.Remove(ri.metafile)
	if err != nil {
		return fmt.Errorf("не удалось удалить файл метаданных %s:%w", ri.metafile, err)
	}

	//Если метаданные удалилли то файл данных может и остаться
	os.Remove(ri.datafile)
	os.Remove(ri.tmpfile)

	return nil
}

// ApplyTemp применяет временный файл как основной файл данных
// Используется после загрузки данных с сервера
func (ri *repositoryItem) ApplyTemp() error {
	err := os.Rename(ri.tmpfile, ri.datafile)
	if err != nil {
		return fmt.Errorf("ошибка получения данных: %w", err)
	}

	return nil
}

// SetMessage устанавливает информационное сообщение для элемента - сделан для установки параметра из других грутин
func (ri *repositoryItem) SetMessage(newMessage string) {
	ri.mu.Lock()
	defer ri.mu.Unlock()

	ri.message = newMessage
}

// SetStatus изменяет статус элемента на новый с проверкой допустимости перехода
// Сохраняет предыдущий статус для возможного восстановления
func (ri *repositoryItem) SetStatus(newStatus itemStatus) error {
	ri.mu.Lock()
	defer ri.mu.Unlock()

	ok := false

	switch newStatus {
	case itemStatusCreating:
		if ri.status == itemStatusNew || ri.status == itemStatusCreated {
			ok = true
		}
	case itemStatusCreated:
		if ri.status == itemStatusCreating {
			ok = true
		}
	case itemStatusSendingCreated:
		if ri.status == itemStatusCreated {
			ok = true
		}
	case itemStatusEditing:
		if ri.status == itemStatusSynced || ri.status == itemStatusEdited {
			ok = true
		}
	case itemStatusEdited:
		if ri.status == itemStatusEditing {
			ok = true
		}
	case itemStatusSendingEdited:
		if ri.status == itemStatusEdited {
			ok = true
		}
	case itemStatusDeleted:
		if ri.status == itemStatusSynced || ri.status == itemStatusEdited || ri.status == itemStatusCreated {
			ok = true
		}
	case itemStatusSendingDeleted:
		if ri.status == itemStatusDeleted {
			ok = true
		}
	case itemStatusReceaving:
		if ri.status == itemStatusNew {
			ok = true
		}
	case itemStatusUpdating:
		if ri.status == itemStatusSynced {
			ok = true
		}
	case itemStatusSynced:
		if ri.status == itemStatusSendingCreated || ri.status == itemStatusSendingEdited || ri.status == itemStatusReceaving || ri.status == itemStatusUpdating {
			ok = true
		}
	case itemStatusConflict:
		if ri.status == itemStatusSendingEdited || ri.status == itemStatusSendingDeleted {
			ok = true
		}
	}

	if ok {
		ri.statusPrev = ri.status
		ri.status = newStatus
		ri.saveMeta()
		return nil
	}

	//Некоторые ошибки возвращаем в читаемом статусе
	switch ri.status {
	case itemStatusConflict:
		return fmt.Errorf("Конфликт состояний между локальной и удаленной версией [%s<>%s]", string(ri.status), string(newStatus))
	case itemStatusError:
		return fmt.Errorf("Данный тип объекта не поддерживается этой версией клиента")
	case itemStatusCreating,
		itemStatusEditing,
		itemStatusReceaving,
		itemStatusUpdating,
		itemStatusSendingCreated,
		itemStatusSendingEdited,
		itemStatusSendingDeleted:
		return fmt.Errorf("Невозможно выполнить операцию. В данный момент объект занят другим процессом")
	default:
		return fmt.Errorf("ошибка смены статуса [%s -> %s]", ri.status, newStatus)
	}

}

// RestorePrevStatus восстанавливает предыдущий статус элемента
// Используется при отмене операций или ошибках
func (ri *repositoryItem) RestorePrevStatus() {
	ri.mu.Lock()
	defer ri.mu.Unlock()

	switch ri.status {
	case itemStatusError, itemStatusNew, itemStatusSynced, itemStatusCreated, itemStatusDeleted, itemStatusEdited, itemStatusConflict:
		//Это стабильные статусы, их менять не нужно
	case itemStatusCreating:
		if ri.statusPrev == itemStatusEmpty {
			ri.status = itemStatusCreated
		} else {
			ri.status = ri.statusPrev
		}
	case itemStatusSendingCreated:
		ri.status = itemStatusCreated
	case itemStatusEditing:
		if ri.statusPrev == itemStatusEmpty {
			ri.status = itemStatusEdited
		} else {
			ri.status = ri.statusPrev
		}
	case itemStatusSendingEdited:
		ri.status = itemStatusEdited
	case itemStatusSendingDeleted:
		ri.status = itemStatusDeleted
	case itemStatusReceaving:
		ri.status = itemStatusNew
	case itemStatusUpdating:
		ri.status = itemStatusSynced
	default:
		logger.Log.Error("ошибка восстановления статуса [%s]", ri.status)
	}

	ri.saveMeta()

}

// GetIfaceObject возвращает интерфейсное представление элемента
//
// Возвращает:
//   - *model.IfaceObject: объект с метаданными элемента
func (ri *repositoryItem) GetIfaceObject() *model.IfaceObject {
	return &model.IfaceObject{
		Id:      ri.id,
		Name:    ri.name,
		Kind:    string(ri.kind),
		Version: ri.version,
		Updated: ri.updated,
		Status:  string(ri.status),
		Message: ri.message,
	}
}

// GetTransportObjectRead возвращает транспортный объект для чтения данных
// Открывает файл данных и возвращает его как io.ReadCloser
func (ri *repositoryItem) GetTransportObjectRead() (*model.TransportObject, error) {

	src, err := os.OpenFile(ri.datafile, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}

	info, err := src.Stat()
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения данных: %w", err)
	}

	size := info.Size()

	tobj := model.TransportObject{
		Id:       int32(ri.id),
		Name:     ri.name,
		Kind:     string(ri.kind),
		Version:  ri.version,
		Client:   ri.client,
		Data:     src,
		DataSize: int32(size),
	}

	return &tobj, nil
}

// GetTransportObjectWrite возвращает транспортный объект для записи данных
// Открывает временный файл для записи и возвращает его как io.WriteCloser
func (ri *repositoryItem) GetTransportObjectWrite() (*model.TransportObject, error) {

	dst, err := os.OpenFile(ri.tmpfile, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть файл данных для записи: %w", err)
	}

	tobj := model.TransportObject{
		Id:      int32(ri.id),
		Name:    ri.name,
		Kind:    string(ri.kind),
		Version: ri.version,
		Client:  ri.client,
		Data:    dst,
	}

	return &tobj, nil
}

// GetTransportObject возвращает транспортный объект без данных (только метаданные)
func (ri *repositoryItem) GetTransportObject() (*model.TransportObject, error) {

	tobj := model.TransportObject{
		Id:      int32(ri.id),
		Name:    ri.name,
		Kind:    string(ri.kind),
		Version: ri.version,
		Client:  ri.client,
	}

	return &tobj, nil
}
