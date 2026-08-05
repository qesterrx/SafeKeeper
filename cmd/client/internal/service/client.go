// Package service предоставляет бизнес-логику и сервисный слой приложения
package service

import (
	"context"
	"fmt"

	"github.com/qesterrx/SafeKeeper/cmd/client/internal/model"
	"github.com/qesterrx/SafeKeeper/cmd/client/internal/transport"
)

// GRPCConnect определяет интерфейс для gRPC-соединения с сервером
// Предоставляет методы для выполнения CRUD-операций над объектами SafeData
type GRPCConnect interface {
	// Close закрывает gRPC-соединение
	Close()
	// NewSafeData создает новый объект SafeData на сервере
	NewSafeData(ctx context.Context, obj *model.TransportObject, setProcessInfo func(string)) error
	// EditSafeData редактирует существующий объект SafeData на сервере
	EditSafeData(ctx context.Context, obj *model.TransportObject, setProcessInfo func(string)) error
	// DelSafeData удаляет объект SafeData на сервере
	DelSafeData(ctx context.Context, obj *model.TransportObject) error
	// GetSafeDataList получает список всех объектов SafeData с сервера
	GetSafeDataList(ctx context.Context) ([]*model.TransportObject, error)
	// GetSafeData получает конкретный объект SafeData с сервера
	GetSafeData(ctx context.Context, obj *model.TransportObject) error
	// ErrNeedReAuth проверяет, требует ли ошибка повторной аутентификации
	ErrNeedReAuth(err error) bool
	// ErrObjConflict проверяет, является ли ошибка конфликтом версий объекта
	ErrObjConflict(err error) bool
}

// Client представляет клиентский сервис для взаимодействия с gRPC-сервером
type Client struct {
	conn *transport.GRPCConnect
}

// NewClient создает новый экземпляр клиента, выполняет аутентификацию на сервере и устанавливает gRPC-соединение
func NewClient(login, password, adress, aesTokenRepo, certCAFile string, clientVer int32) (*Client, error) {
	server, err := transport.NewGRPCServer(certCAFile)
	if err != nil {
		return nil, err
	}

	jwtToken, aesToken, err := server.Login(login, password, adress)
	if err != nil {
		return nil, err
	}

	if aesToken != aesTokenRepo {
		return nil, fmt.Errorf("разные токены шифрования на сервере и клиенте, требуется повторная авторизация")
	}

	conn, err := transport.NewGRPCConnect(adress, jwtToken, clientVer, certCAFile)
	if err != nil {
		return nil, err
	}

	client := Client{
		conn: conn,
	}

	return &client, nil
}

// Close закрывает gRPC-соединение с сервером
func (c *Client) Close() {
	c.conn.Close()
}

// SendNewObj отправляет новый объект на сервер для создания
// Обновляет локальные метаданные объекта (id, version, updated) при успешном создании
//
// Параметры:
//   - ctx: контекст для управления временем выполнения
//   - obj: объект репозитория для отправки
//
// Возвращает:
//   - error: ошибка
func (c *Client) SendNewObj(ctx context.Context, obj *repositoryItem) error {

	tobj, err := obj.GetTransportObjectRead()
	if err != nil {
		return err
	}

	defer func() {
		err = tobj.Data.Close()
	}()

	err = c.conn.NewSafeData(ctx, tobj, obj.SetMessage)
	if err != nil {
		return err
	}

	obj.id = tobj.Id
	obj.version = tobj.Version
	obj.updated = tobj.Updated

	return nil
}

// SendEditObj отправляет измененный объект на сервер для обновления
// Обновляет локальные метаданные объекта (version, updated) при успешном редактировании
//
// Параметры:
//   - ctx: контекст для управления временем выполнения
//   - obj: объект репозитория для отправки
//
// Возвращает:
//   - error: ошибка
func (c *Client) SendEditObj(ctx context.Context, obj *repositoryItem) error {

	tobj, err := obj.GetTransportObjectRead()
	if err != nil {
		return err
	}

	defer func() {
		err = tobj.Data.Close()
	}()

	err = c.conn.EditSafeData(ctx, tobj, obj.SetMessage)
	if err != nil {
		return err
	}

	obj.version = tobj.Version
	obj.updated = tobj.Updated

	return nil
}

// SendDelObj отправляет запрос на удаление объекта на сервере
//
// Параметры:
//   - ctx: контекст для управления временем выполнения
//   - obj: объект репозитория для удаления
//
// Возвращает:
//   - error: ошибка
func (c *Client) SendDelObj(ctx context.Context, obj *repositoryItem) error {

	tobj, err := obj.GetTransportObject()
	if err != nil {
		return err
	}

	err = c.conn.DelSafeData(ctx, tobj)
	if err != nil {
		return err
	}

	return nil
}

// SendGetObjList получает список всех объектов с сервера
//
// Параметры:
//   - ctx: контекст для управления временем выполнения
//
// Возвращает:
//   - []*model.TransportObject: срез транспортных объектов с сервера
//   - error: ошибка
func (c *Client) SendGetObjList(ctx context.Context) ([]*model.TransportObject, error) {

	tobjs, err := c.conn.GetSafeDataList(ctx)
	if err != nil {
		return nil, err
	}

	return tobjs, nil
}

// SendGetObj получает конкретный объект с сервера и сохраняет его в локальный временный файл для последующей обработки
//
// Параметры:
//   - ctx: контекст для управления временем выполнения
//   - obj: объект репозитория для получения данных
//
// Возвращает:
//   - error: ошибка
func (c *Client) SendGetObj(ctx context.Context, obj *repositoryItem) error {

	tobj, err := obj.GetTransportObjectWrite()
	if err != nil {
		return err
	}

	defer func() {
		err = tobj.Data.Close()
		if err == nil {
			err = obj.ApplyTemp()
		}
	}()

	err = c.conn.GetSafeData(ctx, tobj)
	if err != nil {
		return err
	}

	obj.version = tobj.Version
	obj.client = tobj.Client
	obj.updated = tobj.Updated

	return nil
}

// ErrNeedReAuth проверяет, требует ли ошибка повторной аутентификации, прокси-метод к соответствующему методу gRPC-соединения
func (c *Client) ErrNeedReAuth(err error) bool {
	return c.conn.ErrNeedReAuth(err)
}

// ErrObjConflict проверяет, является ли ошибка конфликтом версий объекта, прокси-метод к соответствующему методу gRPC-соединения
func (c *Client) ErrObjConflict(err error) bool {
	return c.conn.ErrObjConflict(err)
}
