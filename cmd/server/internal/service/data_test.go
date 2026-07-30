package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/qesterrx/SafeKeeper/cmd/server/internal/lerrors"
	"github.com/qesterrx/SafeKeeper/cmd/server/internal/model"
)

// MockDataStorage implements DataStorage interface for testing
type MockDataStorage struct {
	GetObjectDescFunc     func(ctx context.Context, id, user int32) (*model.DBObject, error)
	GetObjectDataFunc     func(ctx context.Context, id, user int32) ([]byte, error)
	GetObjectListDescFunc func(ctx context.Context, user int32) ([]*model.DBObject, error)
	NewObjectFunc         func(ctx context.Context, object *model.DBObject) (int32, error)
	UpdateObjectFunc      func(ctx context.Context, object *model.DBObject) error
}

func (m *MockDataStorage) GetObjectDesc(ctx context.Context, id, user int32) (*model.DBObject, error) {
	if m.GetObjectDescFunc != nil {
		return m.GetObjectDescFunc(ctx, id, user)
	}
	return &model.DBObject{
		Id:       id,
		User:     user,
		Name:     "test-object",
		Kind:     "password",
		Data:     []byte("test-data"),
		CheckSum: "abc123",
		Updated:  time.Now(),
		Version:  1,
		Client:   100,
		Deleted:  false,
	}, nil
}

func (m *MockDataStorage) GetObjectData(ctx context.Context, id, user int32) ([]byte, error) {
	if m.GetObjectDataFunc != nil {
		return m.GetObjectDataFunc(ctx, id, user)
	}
	return []byte("test-data"), nil
}

func (m *MockDataStorage) GetObjectListDesc(ctx context.Context, user int32) ([]*model.DBObject, error) {
	if m.GetObjectListDescFunc != nil {
		return m.GetObjectListDescFunc(ctx, user)
	}
	return []*model.DBObject{
		{
			Id:       1,
			User:     user,
			Name:     "object1",
			Kind:     "password",
			CheckSum: "sum1",
			Updated:  time.Now(),
			Version:  1,
			Client:   100,
			Deleted:  false,
		},
		{
			Id:       2,
			User:     user,
			Name:     "object2",
			Kind:     "card",
			CheckSum: "sum2",
			Updated:  time.Now(),
			Version:  1,
			Client:   100,
			Deleted:  false,
		},
	}, nil
}

func (m *MockDataStorage) NewObject(ctx context.Context, object *model.DBObject) (int32, error) {
	if m.NewObjectFunc != nil {
		return m.NewObjectFunc(ctx, object)
	}
	return 100, nil
}

func (m *MockDataStorage) UpdateObject(ctx context.Context, object *model.DBObject) error {
	if m.UpdateObjectFunc != nil {
		return m.UpdateObjectFunc(ctx, object)
	}
	return nil
}

func TestNewDataService(t *testing.T) {
	mockStorage := &MockDataStorage{}
	service, err := NewDataService(mockStorage)
	if err != nil {
		t.Fatalf("NewDataService() error = %v", err)
	}

	if service == nil {
		t.Error("NewDataService() returned nil")
	}
	if service.storage == nil {
		t.Error("Service storage is nil")
	}
	if service.locks == nil {
		t.Error("Service locks map is nil")
	}
}

func TestDataService_NewSafeObject_Success(t *testing.T) {
	mockStorage := &MockDataStorage{
		NewObjectFunc: func(ctx context.Context, object *model.DBObject) (int32, error) {
			if object.User != 123 {
				t.Errorf("Object User = %v, want 123", object.User)
			}
			if object.Version != 1 {
				t.Errorf("Object Version = %v, want 1", object.Version)
			}
			if object.Deleted != false {
				t.Error("Object Deleted should be false")
			}
			return 999, nil
		},
	}

	service, _ := NewDataService(mockStorage)

	obj := &model.SafeObject{
		Name:     "new-object",
		Kind:     "password",
		Data:     []byte("secret-data"),
		CheckSum: "checksum123",
		Client:   100,
	}

	err := service.NewSafeObject(context.Background(), 123, obj)
	if err != nil {
		t.Fatalf("NewSafeObject() error = %v", err)
	}

	if obj.Id != 999 {
		t.Errorf("Object Id = %v, want 999", obj.Id)
	}
	if obj.Version != 1 {
		t.Errorf("Object Version = %v, want 1", obj.Version)
	}
	if obj.Deleted != false {
		t.Error("Object Deleted should be false")
	}
}

func TestDataService_NewSafeObject_StorageError(t *testing.T) {
	mockStorage := &MockDataStorage{
		NewObjectFunc: func(ctx context.Context, object *model.DBObject) (int32, error) {
			return 0, lerrors.NewLEInternalError("database error")
		},
	}

	service, _ := NewDataService(mockStorage)

	obj := &model.SafeObject{
		Name: "test-object",
		Kind: "password",
	}

	err := service.NewSafeObject(context.Background(), 123, obj)
	if err == nil {
		t.Error("NewSafeObject() expected error, got nil")
	}
}

func TestDataService_EditSafeObject_Success(t *testing.T) {
	now := time.Now()
	mockStorage := &MockDataStorage{
		GetObjectDescFunc: func(ctx context.Context, id, user int32) (*model.DBObject, error) {
			return &model.DBObject{
				Id:       id,
				User:     user,
				Name:     "old-name",
				Kind:     "password",
				Data:     []byte("old-data"),
				CheckSum: "old-checksum",
				Updated:  now,
				Version:  1,
				Client:   100,
				Deleted:  false,
			}, nil
		},
		UpdateObjectFunc: func(ctx context.Context, object *model.DBObject) error {
			if object.Version != 2 {
				t.Errorf("Object Version = %v, want 2", object.Version)
			}
			if string(object.Data) != "new-data" {
				t.Errorf("Object Data = %v, want new-data", string(object.Data))
			}
			return nil
		},
	}

	service, _ := NewDataService(mockStorage)

	obj := &model.SafeObject{
		Id:       1,
		Name:     "new-name",
		Kind:     "password",
		Data:     []byte("new-data"),
		CheckSum: "new-checksum",
		Version:  1,
		Client:   100,
	}

	err := service.EditSafeObject(context.Background(), 123, obj)
	if err != nil {
		t.Fatalf("EditSafeObject() error = %v", err)
	}

	if obj.Version != 2 {
		t.Errorf("Object Version = %v, want 2", obj.Version)
	}
	if obj.Updated.IsZero() {
		t.Error("Object Updated should be set")
	}
}

func TestDataService_EditSafeObject_DeletedObject(t *testing.T) {
	mockStorage := &MockDataStorage{
		GetObjectDescFunc: func(ctx context.Context, id, user int32) (*model.DBObject, error) {
			return &model.DBObject{
				Id:      id,
				User:    user,
				Name:    "deleted-object",
				Version: 1,
				Deleted: true,
			}, nil
		},
	}

	service, _ := NewDataService(mockStorage)

	obj := &model.SafeObject{
		Id:      1,
		Version: 1,
		Data:    []byte("new-data"),
	}

	err := service.EditSafeObject(context.Background(), 123, obj)
	if err == nil {
		t.Error("EditSafeObject() expected error for deleted object, got nil")
	}

	var lErr *lerrors.LError
	if errors.As(err, &lErr) {
		if lErr.Code != lerrors.StObjectTooOld {
			t.Errorf("Error code = %v, want %v", lErr.Code, lerrors.StObjectTooOld)
		}
	}
}

func TestDataService_EditSafeObject_VersionConflict(t *testing.T) {
	mockStorage := &MockDataStorage{
		GetObjectDescFunc: func(ctx context.Context, id, user int32) (*model.DBObject, error) {
			return &model.DBObject{
				Id:      id,
				User:    user,
				Name:    "test-object",
				Version: 5, // Server version is newer
				Deleted: false,
			}, nil
		},
	}

	service, _ := NewDataService(mockStorage)

	obj := &model.SafeObject{
		Id:      1,
		Version: 3, // Client has older version
		Data:    []byte("new-data"),
	}

	err := service.EditSafeObject(context.Background(), 123, obj)
	if err == nil {
		t.Error("EditSafeObject() expected version conflict error, got nil")
	}

	var lErr *lerrors.LError
	if errors.As(err, &lErr) {
		if lErr.Code != lerrors.StObjectTooOld {
			t.Errorf("Error code = %v, want %v", lErr.Code, lerrors.StObjectTooOld)
		}
	}
}

func TestDataService_EditSafeObject_StorageError(t *testing.T) {
	mockStorage := &MockDataStorage{
		GetObjectDescFunc: func(ctx context.Context, id, user int32) (*model.DBObject, error) {
			return nil, lerrors.NewLEInternalError("database error")
		},
	}

	service, _ := NewDataService(mockStorage)

	obj := &model.SafeObject{
		Id:      1,
		Version: 1,
		Data:    []byte("new-data"),
	}

	err := service.EditSafeObject(context.Background(), 123, obj)
	if err == nil {
		t.Error("EditSafeObject() expected error, got nil")
	}
}

func TestDataService_DelSafeObject_Success(t *testing.T) {
	now := time.Now()
	mockStorage := &MockDataStorage{
		GetObjectDescFunc: func(ctx context.Context, id, user int32) (*model.DBObject, error) {
			return &model.DBObject{
				Id:      id,
				User:    user,
				Name:    "test-object",
				Version: 1,
				Deleted: false,
				Updated: now,
			}, nil
		},
		UpdateObjectFunc: func(ctx context.Context, object *model.DBObject) error {
			if !object.Deleted {
				t.Error("Object Deleted should be true")
			}
			if object.Version != 1 {
				t.Errorf("Object Version = %v, want 1", object.Version)
			}
			return nil
		},
	}

	service, _ := NewDataService(mockStorage)

	err := service.DelSafeObject(context.Background(), 123, 1, 1)
	if err != nil {
		t.Fatalf("DelSafeObject() error = %v", err)
	}
}

func TestDataService_DelSafeObject_VersionConflict(t *testing.T) {
	mockStorage := &MockDataStorage{
		GetObjectDescFunc: func(ctx context.Context, id, user int32) (*model.DBObject, error) {
			return &model.DBObject{
				Id:      id,
				User:    user,
				Name:    "test-object",
				Version: 5, // Server version is newer
				Deleted: false,
			}, nil
		},
	}

	service, _ := NewDataService(mockStorage)

	err := service.DelSafeObject(context.Background(), 123, 1, 3)
	if err == nil {
		t.Error("DelSafeObject() expected version conflict error, got nil")
	}

	var lErr *lerrors.LError
	if errors.As(err, &lErr) {
		if lErr.Code != lerrors.StObjectTooOld {
			t.Errorf("Error code = %v, want %v", lErr.Code, lerrors.StObjectTooOld)
		}
	}
}

func TestDataService_DelSafeObject_ObjectNotFound(t *testing.T) {
	mockStorage := &MockDataStorage{
		GetObjectDescFunc: func(ctx context.Context, id, user int32) (*model.DBObject, error) {
			return nil, lerrors.NewLEObjectNotFound("object not found")
		},
	}

	service, _ := NewDataService(mockStorage)

	err := service.DelSafeObject(context.Background(), 123, 999, 1)
	if err == nil {
		t.Error("DelSafeObject() expected error, got nil")
	}
}

func TestDataService_GetSafeObject_Success(t *testing.T) {
	now := time.Now()
	mockStorage := &MockDataStorage{
		GetObjectDescFunc: func(ctx context.Context, id, user int32) (*model.DBObject, error) {
			return &model.DBObject{
				Id:       id,
				User:     user,
				Name:     "test-object",
				Kind:     "password",
				CheckSum: "checksum123",
				Updated:  now,
				Version:  1,
				Client:   100,
				Deleted:  false,
			}, nil
		},
		GetObjectDataFunc: func(ctx context.Context, id, user int32) ([]byte, error) {
			return []byte("encrypted-data"), nil
		},
	}

	service, _ := NewDataService(mockStorage)

	obj, err := service.GetSafeObject(context.Background(), 123, 1)
	if err != nil {
		t.Fatalf("GetSafeObject() error = %v", err)
	}

	if obj.Id != 1 {
		t.Errorf("Object Id = %v, want 1", obj.Id)
	}
	if obj.Name != "test-object" {
		t.Errorf("Object Name = %v, want test-object", obj.Name)
	}
	if string(obj.Data) != "encrypted-data" {
		t.Errorf("Object Data = %v, want encrypted-data", string(obj.Data))
	}
	if obj.Deleted != false {
		t.Error("Object Deleted should be false")
	}
}

func TestDataService_GetSafeObject_Deleted(t *testing.T) {
	mockStorage := &MockDataStorage{
		GetObjectDescFunc: func(ctx context.Context, id, user int32) (*model.DBObject, error) {
			return &model.DBObject{
				Id:      id,
				User:    user,
				Name:    "deleted-object",
				Deleted: true,
			}, nil
		},
	}

	service, _ := NewDataService(mockStorage)

	_, err := service.GetSafeObject(context.Background(), 123, 1)
	if err == nil {
		t.Error("GetSafeObject() expected error for deleted object, got nil")
	}

	var lErr *lerrors.LError
	if errors.As(err, &lErr) {
		if lErr.Code != lerrors.StObjectNotFound {
			t.Errorf("Error code = %v, want %v", lErr.Code, lerrors.StObjectNotFound)
		}
	}
}

func TestDataService_GetSafeObject_NotFound(t *testing.T) {
	mockStorage := &MockDataStorage{
		GetObjectDescFunc: func(ctx context.Context, id, user int32) (*model.DBObject, error) {
			return nil, lerrors.NewLEObjectNotFound("object not found")
		},
	}

	service, _ := NewDataService(mockStorage)

	_, err := service.GetSafeObject(context.Background(), 123, 999)
	if err == nil {
		t.Error("GetSafeObject() expected error, got nil")
	}
}

func TestDataService_GetSafeObjectList_Success(t *testing.T) {
	now := time.Now()
	mockStorage := &MockDataStorage{
		GetObjectListDescFunc: func(ctx context.Context, user int32) ([]*model.DBObject, error) {
			return []*model.DBObject{
				{
					Id:       1,
					User:     user,
					Name:     "object1",
					Kind:     "password",
					CheckSum: "sum1",
					Updated:  now,
					Version:  1,
					Client:   100,
					Deleted:  false,
				},
				{
					Id:       2,
					User:     user,
					Name:     "object2",
					Kind:     "card",
					CheckSum: "sum2",
					Updated:  now,
					Version:  2,
					Client:   100,
					Deleted:  false,
				},
				{
					Id:       3,
					User:     user,
					Name:     "object3",
					Kind:     "text",
					CheckSum: "sum3",
					Updated:  now,
					Version:  1,
					Client:   100,
					Deleted:  true, // Should be included but flagged
				},
			}, nil
		},
	}

	service, _ := NewDataService(mockStorage)

	objs, err := service.GetSafeObjectList(context.Background(), 123)
	if err != nil {
		t.Fatalf("GetSafeObjectList() error = %v", err)
	}

	if len(objs) != 3 {
		t.Errorf("List length = %v, want 3", len(objs))
	}

	if objs[0].Id != 1 {
		t.Errorf("First object Id = %v, want 1", objs[0].Id)
	}
	if objs[1].Id != 2 {
		t.Errorf("Second object Id = %v, want 2", objs[1].Id)
	}
	if objs[2].Id != 3 {
		t.Errorf("Third object Id = %v, want 3", objs[2].Id)
	}
	if !objs[2].Deleted {
		t.Error("Third object Deleted should be true")
	}
}

func TestDataService_GetSafeObjectList_Empty(t *testing.T) {
	mockStorage := &MockDataStorage{
		GetObjectListDescFunc: func(ctx context.Context, user int32) ([]*model.DBObject, error) {
			return []*model.DBObject{}, nil
		},
	}

	service, _ := NewDataService(mockStorage)

	objs, err := service.GetSafeObjectList(context.Background(), 123)
	if err != nil {
		t.Fatalf("GetSafeObjectList() error = %v", err)
	}

	if len(objs) != 0 {
		t.Errorf("List length = %v, want 0", len(objs))
	}
}

func TestDataService_GetSafeObjectList_StorageError(t *testing.T) {
	mockStorage := &MockDataStorage{
		GetObjectListDescFunc: func(ctx context.Context, user int32) ([]*model.DBObject, error) {
			return nil, lerrors.NewLEInternalError("database error")
		},
	}

	service, _ := NewDataService(mockStorage)

	_, err := service.GetSafeObjectList(context.Background(), 123)
	if err == nil {
		t.Error("GetSafeObjectList() expected error, got nil")
	}
}

func TestDataService_ConcurrentAccess(t *testing.T) {
	mockStorage := &MockDataStorage{
		GetObjectDescFunc: func(ctx context.Context, id, user int32) (*model.DBObject, error) {
			time.Sleep(10 * time.Millisecond) // Simulate DB delay
			return &model.DBObject{
				Id:      id,
				User:    user,
				Name:    "test-object",
				Version: 1,
				Deleted: false,
			}, nil
		},
		UpdateObjectFunc: func(ctx context.Context, object *model.DBObject) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	}

	service, _ := NewDataService(mockStorage)

	// Запускаем несколько горутин для редактирования одного объекта
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			obj := &model.SafeObject{
				Id:      1,
				Version: 1,
				Data:    []byte("new-data"),
			}
			err := service.EditSafeObject(context.Background(), 123, obj)
			if err != nil {
				// Для этого теста игнорируем ошибки (может быть версионный конфликт)
				// Главное - не должно быть паники
			}
			done <- true
		}()
	}

	// Ждём завершения всех горутин
	for i := 0; i < 5; i++ {
		<-done
	}
	// Если тест дошел сюда - значит не было паники от конкурентного доступа
}
