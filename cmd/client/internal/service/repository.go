// Package service предоставляет бизнес-логику и сервисный слой приложения
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/qesterrx/SafeKeeper/cmd/client/internal/config"
	"github.com/qesterrx/SafeKeeper/cmd/client/internal/model"
	"github.com/qesterrx/SafeKeeper/internal/logger"
	"golang.org/x/sync/errgroup"
)

// Transport определяет интерфейс для транспортного слоя клиента
// Предоставляет методы для выполнения операций с объектами SafeData
type Transport interface {
	// Close закрывает транспортное соединение
	Close()
	// SendNewObj отправляет новый объект на сервер
	SendNewObj(ctx context.Context, obj *repositoryItem) error
	// SendEditObj отправляет измененный объект на сервер
	SendEditObj(ctx context.Context, obj *repositoryItem) error
	// SendDelObj отправляет запрос на удаление объекта
	SendDelObj(ctx context.Context, obj *repositoryItem) error
	// SendGetObjList получает список объектов с сервера
	SendGetObjList(ctx context.Context) ([]*model.TransportObject, error)
	// SendGetObj получает конкретный объект с сервера
	SendGetObj(ctx context.Context, obj *repositoryItem) error
	// ErrNeedReAuth проверяет необходимость повторной аутентификации
	ErrNeedReAuth(err error) bool
	// ErrObjConflict проверяет конфликт версий
	ErrObjConflict(err error) bool
}

// Reposityory представляет локальный репозиторий объектов SafeData
type Reposityory struct {
	cfg    *config.Configuration
	repo   *ListRepositoryItem
	aeskey []byte
	idx    int32
	items  map[int32]*repositoryItem

	pswd string

	mu sync.Mutex

	client Transport
}

// NewLocalReposityory создает новый экземпляр локального репозитория
// Загружает существующие метаданные из файловой системы и восстанавливает состояние объектов
// Параметры:
//   - repo: элемент репозитория с настройками подключения
//   - pswd: пароль пользователя для расшифровки AES-ключа
func NewLocalReposityory(repo *model.IfaceRepoItem, pswd string, cfg *config.Configuration) (*Reposityory, error) {

	repoItem := &ListRepositoryItem{
		Name:     repo.Name,
		Host:     repo.Host,
		Port:     repo.Port,
		Path:     repo.Path,
		Login:    repo.Login,
		Status:   repo.Status,
		AESToken: repo.AESToken,
	}

	aesKey, err := repoItem.CheckCredentials(pswd)
	if err != nil {
		return nil, err
	}

	files, err := os.ReadDir(repoItem.Path)
	if err != nil {
		return nil, fmt.Errorf("Ошибка чтения каталога: %w\n", err)
	}

	r := Reposityory{
		repo:   repoItem,
		pswd:   pswd,
		aeskey: aesKey,
		items:  map[int32]*repositoryItem{},
		idx:    0,
		cfg:    cfg,
	}

	//Загружаем локальные метаданные
	for _, file := range files {
		if !file.IsDir() {
			filename := file.Name()
			if strings.HasPrefix(filename, "meta_") {

				data, err := os.ReadFile(filepath.Join(r.repo.Path, filename))
				if err != nil {
					return nil, fmt.Errorf("не удалось прочитать файл метаданных %s: %w", filename, err)
				}

				meta := model.MetaObject{}
				err = json.Unmarshal(data, &meta)
				if err != nil {
					return nil, fmt.Errorf("файл метаданных %s имеет неподдерживаемый формат: %w", filename, err)
				}

				ri := restoreRepositoryItemFromMeta(r.repo.Path, filename, &meta)
				if ri != nil {
					i, ok := r.items[ri.id]
					if ok {
						return nil, fmt.Errorf("файл метаданных %s содержит такой же ИД как и файл %s", filename, i.metafile)
					} else {
						r.items[ri.id] = ri

						if ri.id < r.idx {
							r.idx = ri.id
						}
					}

				}

			}
		}
	}

	return &r, nil
}

// GetItemList возвращает отсортированный список всех объектов в локальном репозитории
// Сортировка выполняется по типу объекта (Kind), а внутри типа - по ID
//
// Возвращает:
//   - []*model.IfaceObject: срез интерфейсных объектов
//   - error: всегда nil, оставлен для совместимости
func (r *Reposityory) GetItemList() ([]*model.IfaceObject, error) {

	r.mu.Lock()
	defer r.mu.Unlock()

	items := []*model.IfaceObject{}

	for _, item := range r.items {
		items = append(items, item.GetIfaceObject())
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind == items[j].Kind {
			return items[i].Id < items[j].Id
		}
		return items[i].Kind < items[j].Kind
	})

	return items, nil
}

// NewItem создает новый объект в локальном репозитории с указанными именем и типом
// Объект создается с отрицательным ID, что указывает на то, что он еще не синхронизирован с сервером
//
// Параметры:
//   - name: имя нового объекта
//   - kind: тип объекта (CARD, PASSWORD, TEXT, BIN)
//
// Возвращает:
//   - *model.IfaceObject: интерфейсное представление созданного объекта
//   - error: ошибка
func (r *Reposityory) NewItem(name string, kind string) (*model.IfaceObject, error) {

	r.mu.Lock()
	defer r.mu.Unlock()

	r.idx--
	i, err := newRepositoryItem(r.idx, name, kind, r.repo.Path)
	if err != nil {
		return nil, err
	}

	r.items[r.idx] = i

	err = i.SetStatus(itemStatusCreating)
	if err != nil {
		i.Delete()
		return nil, err
	}

	return i.GetIfaceObject(), nil
}

// EditItem подготавливает объект для редактирования. Изменяет статус объекта, расшифровывает данные и возвращает их вместе с метаданными
// Параметры:
//   - id: идентификатор объекта для редактирования
//
// Возвращает:
//   - *model.IfaceObject: метаданные объекта
//   - []byte: расшифрованные данные объекта
//   - error: ошибка
func (r *Reposityory) EditItem(id int32) (*model.IfaceObject, []byte, error) {

	r.mu.Lock()
	defer r.mu.Unlock()

	i, ok := r.items[id]
	if !ok {
		return nil, nil, fmt.Errorf("Объект с id %d не найден", id)
	}

	var err error

	//Ставим статус что начинаем редактирование в зависимости от текущего статуса объекта
	if i.status == itemStatusCreated {
		err = i.SetStatus(itemStatusCreating)
	} else {
		err = i.SetStatus(itemStatusEditing)
	}

	if err != nil {
		return nil, nil, err
	}

	data := &bytes.Buffer{}
	err = i.ReadEncryptData(data, r.aeskey)
	if err != nil {
		i.SetMessage(err.Error())
		i.RestorePrevStatus()
		return nil, nil, err
	}

	return i.GetIfaceObject(), data.Bytes(), nil
}

// CancelOperation отменяет текущую операцию над объектом, восстанавливает предыдущий статус объекта
// Параметры:
//   - id: идентификатор объекта
//
// Возвращает:
//   - error: ошибка
func (r *Reposityory) CancelOperation(id int32) error {

	r.mu.Lock()
	defer r.mu.Unlock()

	i, ok := r.items[id]
	if !ok {
		return nil
	}

	i.RestorePrevStatus()
	i.SetMessage("")

	if i.status == itemStatusNew {
		err := i.Delete()
		if err != nil {
			return err
		}
		delete(r.items, id)
		return nil
	}

	return nil
}

// UpdItem обновляет данные существующего объекта. Шифрует переданные данные и сохраняет их в файл, обновляет статус объекта
// Параметры:
//   - id: идентификатор объекта
//   - data: байтовый срез с новыми данными
//
// Возвращает:
//   - error: ошибка
func (r *Reposityory) UpdItem(id int32, data []byte) error {

	r.mu.Lock()
	defer r.mu.Unlock()

	i, ok := r.items[id]
	if !ok {
		return fmt.Errorf("Объект с id %d не найден", id)
	}

	reader := bytes.NewReader(data)
	length := len(data)
	err := i.WriteEncryptData(reader, int64(length), r.aeskey)
	if err != nil {
		i.SetMessage(err.Error())
		i.RestorePrevStatus()
		return err
	}

	if i.status == itemStatusCreating {
		err = i.SetStatus(itemStatusCreated)
	} else {
		err = i.SetStatus(itemStatusEdited)
	}

	if err != nil {
		return err
	}

	return nil

}

// DelItem удаляет объект из локального репозитория
// Если объект еще не был синхронизирован с сервером (id < 0), удаляет его локально без вызова сервера
// Иначе помечает объект на удаление для последующей синхронизации
//
// Параметры:
//   - id: идентификатор объекта
//
// Возвращает:
//   - error: ошибка
func (r *Reposityory) DelItem(id int32) error {

	r.mu.Lock()
	defer r.mu.Unlock()

	i, ok := r.items[id]
	if !ok {
		return nil
	}

	//Если запись еще не отправлялась то удалить можно без вызова сервера
	if i.id < 0 {

		err := i.SetStatus(itemStatusSendingDeleted)
		if err != nil {
			return err
		}

		err = i.Delete()
		if err != nil {
			i.SetMessage(err.Error())
			return err
		}

		delete(r.items, id)
		//i = nil

	} else {
		//Ставим статус что запись нужно удалить
		err := i.SetStatus(itemStatusDeleted)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetTempFilename возвращает путь к временному файлу объекта
// Используется для открытия BIN файла
//
// Параметры:
//   - id: идентификатор объекта
//
// Возвращает:
//   - string: путь к временному файлу
//   - error: ошибка
func (r *Reposityory) GetTempFilename(id int32) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	i, ok := r.items[id]
	if !ok {
		return "", fmt.Errorf("Объект с id %d не найден", id)
	}

	return i.tmpfile, nil

}

// BackgroundProcess запускает фоновый процесс синхронизации репозиторияс сервером. Обрабатывает отправку локальных изменений и загрузку удаленных.
//
// Параметры:
//   - ctx: контекст для управления временем жизни процесса
//   - interval: интервал между циклами синхронизации
//
// Возвращает:
//   - error: ошибка
func (r *Reposityory) BackgroundProcess(ctx context.Context, interval time.Duration) error {

	logger.Log.Debug("Reposityory.BackgroundProcess START")
	defer logger.Log.Debug("Reposityory.BackgroundProcess STOP")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Log.Debug("StartRepositoryBackgReposityory.BackgroundProcessround ctx.Done")
			return nil
		case <-ticker.C:
			if r.client == nil {
				client, err := NewClient(r.repo.Login, r.pswd, r.repo.GetAdress(), r.repo.AESToken, r.cfg.CertCA, r.cfg.ClientVersion)

				if err != nil {
					logger.Log.Info("Can't connect to server %s: %v", r.repo.GetAdress(), err)
					r.client = nil
					continue
				}

				r.client = client

				objsChan := make(chan *repositoryItem, 50)

				g, ctxIn := errgroup.WithContext(ctx)

				g.Go(func() error {
					return r.backgroundWriter(ctxIn, objsChan)
				})

				g.Go(func() error {
					return r.backgroundReader(ctxIn, objsChan)
				})

				g.Wait()
				close(objsChan)

				for obj := range objsChan {
					obj.RestorePrevStatus()
				}

				r.client.Close()
				r.client = nil

			}
		}
	}

}

// backgroundWriter выполняет фоновую запись (отправку) объектов на сервер.
// Обрабатывает объекты со статусами, требующими отправки
//
// Параметры:
//   - ctx: контекст для управления временем жизни
//   - interval: интервал между проверками
//   - objs: канал для передачи объектов в обработчик чтения
//
// Возвращает:
//   - error: ошибка
func (r *Reposityory) backgroundWriter(ctx context.Context, objs chan *repositoryItem) error {

	logger.Log.Debug("Reposityory.backgroundWriter START")
	defer logger.Log.Debug("BackgReposityory.backgroundWriterroundWriter STOP")

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Log.Debug("Reposityory.backgroundWriter ctx.Done")
			return nil
		case <-ticker.C:
			//Проверяем локальные статусы

			func() {

				r.mu.Lock()
				defer r.mu.Unlock()

				for _, i := range r.items {
					var err error
					push := false
					switch i.status {
					case itemStatusCreated:
						err = i.SetStatus(itemStatusSendingCreated)
						push = true
					case itemStatusEdited:
						err = i.SetStatus(itemStatusSendingEdited)
						push = true
					case itemStatusDeleted:
						err = i.SetStatus(itemStatusSendingDeleted)
						push = true
					case itemStatusUpdating:
						push = true
					case itemStatusReceaving:
						push = true
					}

					if push {
						if err != nil {
							i.RestorePrevStatus()
							i.SetMessage(err.Error())
						}

						objs <- i
					}

				}

			}()

			remItems, err := r.client.SendGetObjList(ctx)
			if err != nil {
				logger.Log.ForwardError(err)
				if r.client.ErrNeedReAuth(err) {
					return err
				}
			}

			err = func() error {
				r.mu.Lock()
				defer r.mu.Unlock()

				for _, ri := range remItems {
					i, ok := r.items[ri.Id]

					if !ok {
						if !ri.Deleted {
							//На сервере есть объект которого нет локально
							i, err := newRepositoryItem(ri.Id, ri.Name, ri.Kind, r.repo.Path)
							if err != nil {
								logger.Log.ForwardError(err)
							}

							r.items[ri.Id] = i

							err = i.SetStatus(itemStatusReceaving)
							if err != nil {
								i.Delete()
								logger.Log.ForwardError(err)
							}
						}
					} else {
						if ri.Deleted {
							//Объект удален на сервере
							err := i.Delete()
							if err != nil {
								i.SetMessage(err.Error())
							} else {
								delete(r.items, i.id)
							}
						} else {
							if ri.Version > i.version && i.status == itemStatusSynced {
								//На сервере более свежая версия
								err := i.SetStatus(itemStatusUpdating)
								if err != nil {
									i.SetMessage(err.Error())
								}
							}
						}

					}
				}

				return nil
			}()

			if err != nil {
				logger.Log.ForwardError(err)
			}

		}
	}
}

// backgroundReader выполняет фоновое чтение и обработку объектов из канала.
// Отправляет объекты на сервер (создание, редактирование, удаление) и получает данные с сервера (обновление, получение).
//
// Параметры:
//   - ctx: контекст для управления временем жизни
//   - objs: канал для получения объектов на обработку
//
// Возвращает:
//   - error: ошибка
func (r *Reposityory) backgroundReader(ctx context.Context, objs chan *repositoryItem) error {

	logger.Log.Debug("BackgrouReposityory.backgroundReader START")
	defer logger.Log.Debug("BackgrouReposityory.backgroundReader STOP")

	for {
		select {
		case <-ctx.Done():
			logger.Log.Debug("BackgrouReposityory.backgroundReader ctx.Done")
			return nil
		case obj, ok := <-objs:

			if !ok {
				return fmt.Errorf("BackgrouReposityory.backgroundReader чтение из закрытого канала")
			}

			logger.Log.Debug("Из очереди взят объект %d со статусом %s", obj.id, obj.status)

			switch obj.status {
			case itemStatusSendingCreated:

				oldId := obj.id

				err := r.client.SendNewObj(ctx, obj)
				if err != nil {
					obj.SetMessage(err.Error())
					obj.RestorePrevStatus()

					if r.client.ErrNeedReAuth(err) {
						return err
					}

					continue
				}

				err = obj.SetStatus(itemStatusSynced)
				if err != nil {
					obj.SetMessage(err.Error())
					obj.RestorePrevStatus()
					continue
				}

				if oldId != obj.id {

					moveItem := func(oldId, newId int32) error {
						r.mu.Lock()
						defer r.mu.Unlock()

						i, ok := r.items[oldId]
						if !ok {
							return fmt.Errorf("Объект с ID %d не найден", oldId)
						}

						r.items[newId] = i
						delete(r.items, oldId)

						return nil
					}

					err := moveItem(oldId, obj.id)
					if err != nil {
						obj.SetMessage(err.Error())
					}
				}

			case itemStatusSendingEdited:

				err := r.client.SendEditObj(ctx, obj)
				if err != nil {
					obj.SetMessage(err.Error())
					obj.RestorePrevStatus()

					if r.client.ErrNeedReAuth(err) {
						return err
					}

					if r.client.ErrObjConflict(err) {
						err = obj.SetStatus(itemStatusConflict)
						if err != nil {
							obj.SetMessage(err.Error())
							obj.RestorePrevStatus()
						}
					}

					continue
				}

				err = obj.SetStatus(itemStatusSynced)
				if err != nil {
					obj.SetMessage(err.Error())
					obj.RestorePrevStatus()
				}

			case itemStatusSendingDeleted:

				err := r.client.SendDelObj(ctx, obj)
				if err != nil {
					obj.SetMessage(err.Error())
					obj.RestorePrevStatus()

					if r.client.ErrNeedReAuth(err) {
						return err
					}

					if r.client.ErrObjConflict(err) {
						err = obj.SetStatus(itemStatusConflict)
						if err != nil {
							obj.SetMessage(err.Error())
							obj.RestorePrevStatus()
						}
					}

					continue
				}

				delItem := func(id int32) error {
					r.mu.Lock()
					defer r.mu.Unlock()

					i, ok := r.items[id]
					if !ok {
						return fmt.Errorf("Объект с ID %d не найден", id)
					}

					err := i.Delete()
					if err != nil {
						return err
					}

					delete(r.items, id)

					return nil
				}

				err = delItem(obj.id)
				if err != nil {
					obj.SetMessage(err.Error())
					obj.RestorePrevStatus()
				}

			case itemStatusReceaving, itemStatusUpdating:

				err := r.client.SendGetObj(ctx, obj)
				if err != nil {
					obj.SetMessage(err.Error())
					obj.RestorePrevStatus()

					if r.client.ErrNeedReAuth(err) {
						return err
					}

					if r.client.ErrObjConflict(err) {
						err = obj.SetStatus(itemStatusConflict)
						if err != nil {
							obj.SetMessage(err.Error())
							obj.RestorePrevStatus()
						}
					}

					continue
				}

				err = obj.SetStatus(itemStatusSynced)
				if err != nil {
					obj.SetMessage(err.Error())
					obj.RestorePrevStatus()
				}

			default:
				obj.SetMessage("unimplemented operation")
			}
		}
	}

}
