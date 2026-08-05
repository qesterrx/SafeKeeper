// Package service реализует бизнес-логику (сервисный слой) приложения SafeKeeper
// В данном файле реализован сервис управления защищёнными объектами (DataService), отвечающий за CRUD-операции с данными пользователей

package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/qesterrx/SafeKeeper/cmd/server/internal/lerrors"
	"github.com/qesterrx/SafeKeeper/cmd/server/internal/model"
)

// DataStorage определяет интерфейс для работы с хранилищем защищённых объектов
// Реализует все необходимые операции для управления данными пользователей
//
//go:generate mockery --name=DataStorage --output=mocks --outpkg=mocks --filename=data_mock.go
type DataStorage interface {
	// GetObjectDesc возвращает описание объекта без данных (только метаданные)
	GetObjectDesc(ctx context.Context, id, user int32) (*model.DBObject, error)

	// GetObjectData возвращает только зашифрованные данные объекта
	GetObjectData(ctx context.Context, id, user int32) ([]byte, error)

	// GetObjectListDesc возвращает список описаний всех объектов пользователя
	GetObjectListDesc(ctx context.Context, user int32) ([]*model.DBObject, error)

	// NewObject создаёт новый объект в базе данных
	NewObject(ctx context.Context, object *model.DBObject) (int32, error)

	// UpdateObject обновляет существующий объект в базе данных
	UpdateObject(ctx context.Context, object *model.DBObject) error
}

// DataService реализует бизнес-логику управления защищёнными объектами пользователей
type DataService struct {
	storage DataStorage

	locks   map[int32]*sync.Mutex // Карта мьютексов для каждого пользователя
	mxLocks sync.Mutex            // Мьютекс для защиты доступа к карте locks
}

// NewDataService создаёт новый экземпляр DataService
// Параметры:
//   - storage: реализация интерфейса DataStorage
//
// Возвращает:
//   - *DataService: указатель на созданный сервис
//   - error: ошибка инициализации (в текущей реализации всегда nil)
func NewDataService(storage DataStorage) (*DataService, error) {

	//Сделаем мапу защелок, ключем будет выступать ИД пользователя
	l := make(map[int32]*sync.Mutex)

	return &DataService{storage: storage, locks: l}, nil
}

// NewSafeObject создаёт новый защищённый объект для пользователя
// Параметры:
//   - ctx: контекст выполнения
//   - user: идентификатор пользователя-владельца
//   - obj: объект SafeObject для сохранения (без ID, будет заполнен автоматически)
//
// Возвращает:
//   - error: ошибка при сохранении в базе данных
func (ds *DataService) NewSafeObject(ctx context.Context, user int32, obj *model.SafeObject) error {

	obj.Version = 1
	obj.Updated = time.Now()
	obj.Deleted = false

	dbObj := obj.TranslateToDBObject(user)

	id, err := ds.storage.NewObject(ctx, dbObj)
	if err != nil {
		return err
	}

	obj.Id = id

	return nil
}

// DelSafeObject выполняет мягкое удаление защищённого объекта
// Параметры:
//   - ctx: контекст выполнения
//   - user: идентификатор пользователя-владельца
//   - objId: идентификатор удаляемого объекта
//   - objVersion: версия объекта из клиентского запроса (для проверки)
//
// Возвращает:
//   - error: ошибка при проверке версии, получении объекта или обновлении БД
//   - lerrors.ErrObjectTooOld: если версия объекта на сервере новее переданной
func (ds *DataService) DelSafeObject(ctx context.Context, user int32, objId int32, objVersion int32) error {

	//Контроль что изменение данных по объекту идет только в одном потоке
	ds.mxLocks.Lock()
	objLock, exists := ds.locks[objId]

	if !exists {
		objLock = &sync.Mutex{}
		ds.locks[objId] = objLock
	}

	ds.mxLocks.Unlock() //освобождаем общий мьютекс
	objLock.Lock()      //занимаем мьютекс объекта
	defer objLock.Unlock()

	dbObj, err := ds.storage.GetObjectDesc(ctx, objId, user)
	if err != nil {
		return err
	}

	if dbObj.Version > objVersion {
		return lerrors.NewLEObjectTooOld(nil, fmt.Sprintf("объект на сервере более свежий %d>%d", dbObj.Version, objVersion))
	}

	dbObj.Data = nil
	dbObj.Deleted = true
	dbObj.Updated = time.Now()

	err = ds.storage.UpdateObject(ctx, dbObj)
	if err != nil {
		return err
	}

	return nil

}

// EditSafeObject редактирует существующий защищённый объект
// Параметры:
//   - ctx: контекст выполнения
//   - user: идентификатор пользователя-владельца
//   - obj: объект SafeObject с обновлёнными данными (должен содержать ID и версию)
//
// Возвращает:
//   - error: ошибка при проверке версии, получении объекта или обновлении БД
//   - lerrors.ErrObjectTooOld: если версия объекта на сервере новее переданной
//   - lerrors.ErrObjectNotFound: если объект был удалён ранее
func (ds *DataService) EditSafeObject(ctx context.Context, user int32, obj *model.SafeObject) error {

	//Контроль что изменение данных по объекту идет только в одном потоке
	ds.mxLocks.Lock()
	objLock, exists := ds.locks[obj.Id]

	if !exists {
		objLock = &sync.Mutex{}
		ds.locks[obj.Id] = objLock
	}

	ds.mxLocks.Unlock() //освобождаем общий мьютекс
	objLock.Lock()      //занимаем мьютекс объекта
	defer objLock.Unlock()

	dbObjs, err := ds.storage.GetObjectDesc(ctx, obj.Id, user)
	if err != nil {
		return err
	}

	if dbObjs.Deleted {
		return lerrors.NewLEObjectTooOld(nil, "объект на сервере удален")
	}

	if dbObjs.Version > obj.Version {
		return lerrors.NewLEObjectTooOld(nil, fmt.Sprintf("объект на сервере более свежий %d>%d", dbObjs.Version, obj.Version))
	}

	dbObjs.Version = dbObjs.Version + 1
	dbObjs.Updated = time.Now()
	dbObjs.Data = obj.Data

	err = ds.storage.UpdateObject(ctx, dbObjs)
	if err != nil {
		return err
	}

	obj.Version = dbObjs.Version
	obj.Updated = dbObjs.Updated

	return nil

}

// GetSafeObject получает полные данные защищённого объекта по его идентификатору
// Параметры:
//   - ctx: контекст выполнения
//   - user: идентификатор пользователя-владельца
//   - objId: идентификатор запрашиваемого объекта
//
// Возвращает:
//   - *model.SafeObject: указатель на объект с метаданными и зашифрованными данными
//   - error: ошибка при получении из БД или если объект удалён
//   - lerrors.ErrObjectNotFound: если объект не найден или помечен как удалённый
func (ds *DataService) GetSafeObject(ctx context.Context, user int32, objId int32) (*model.SafeObject, error) {

	dbObjs, err := ds.storage.GetObjectDesc(ctx, objId, user)
	if err != nil {
		return nil, err
	}

	if dbObjs.Deleted {
		return nil, lerrors.NewLEObjectNotFound(nil, fmt.Sprintf("Объект [%d] на сервере удален", objId))
	}

	obj := dbObjs.TranslateToSafeObject()

	obj.Data, err = ds.storage.GetObjectData(ctx, objId, user)
	if err != nil {
		return nil, err
	}

	return obj, nil

}

// GetSafeObjectList получает список всех активных защищённых объектов пользователя
// Возвращает только метаданные объектов без зашифрованных данных (поле Data = nil)
// Параметры:
//   - ctx: контекст выполнения
//   - user: идентификатор пользователя-владельца
//
// Возвращает:
//   - []*model.SafeObject: слайс указателей на объекты (без данных)
//   - error: ошибка при получении списка из базы данных
func (ds *DataService) GetSafeObjectList(ctx context.Context, user int32) ([]*model.SafeObject, error) {

	res := []*model.SafeObject{}

	dbObjs, err := ds.storage.GetObjectListDesc(ctx, user)
	if err != nil {
		return nil, err
	}

	for _, dbObj := range dbObjs {
		obj := dbObj.TranslateToSafeObject()
		res = append(res, obj)
	}

	return res, nil

}
