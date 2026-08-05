package service

import (
	"context"
	"testing"
	"time"

	"github.com/qesterrx/SafeKeeper/cmd/server/internal/lerrors"
	"github.com/qesterrx/SafeKeeper/cmd/server/internal/model"
	"github.com/qesterrx/SafeKeeper/cmd/server/internal/service/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewSafeObject(t *testing.T) {
	// Создаем мок
	mockStorage := mocks.NewDataStorage(t)

	obj := model.SafeObject{
		Name:     "name",
		Kind:     "kind",
		Data:     []byte("data"),
		CheckSum: "1",
		Client:   int32(1),
	}

	// Настраиваем ожидания
	mockStorage.On("NewObject", mock.Anything, mock.Anything).Return(int32(1), nil)

	// Создаем сервис
	service, err := NewDataService(mockStorage)
	require.NoError(t, err, "NewDataService() error")

	// Выполняем тестируемую функцию

	err = service.NewSafeObject(context.Background(), int32(1), &obj)

	// Проверяем результат
	require.NoError(t, err, "NewSafeObject() error")
	require.Equal(t, int32(1), obj.Id)
	require.Equal(t, int32(1), obj.Version)
	require.False(t, obj.Deleted)

	// Проверяем, что все ожидания выполнены
	mockStorage.AssertExpectations(t)
}

func TestEditSafeObject(t *testing.T) {

	now := time.Now()

	type testCase struct {
		name        string
		user        int32
		objIn       *model.SafeObject
		objOut      *model.SafeObject
		objDescOut  *model.DBObject
		errDescCode lerrors.Status
		errUpdCode  lerrors.Status
		errCode     lerrors.Status
	}

	tests := []testCase{
		{name: "success",
			user:        int32(1),
			objIn:       &model.SafeObject{Id: int32(1), Version: int32(1), Deleted: false, Updated: now},
			objDescOut:  &model.DBObject{Id: int32(1), User: int32(1), Version: int32(1), Deleted: false},
			objOut:      &model.SafeObject{Version: int32(2), Updated: time.Now()},
			errDescCode: -1,
			errUpdCode:  -1,
			errCode:     -1,
		},
		{name: "GetObjectDesc_StObjectNotFound",
			user:        int32(1),
			objIn:       &model.SafeObject{Id: int32(1), Version: int32(1), Deleted: false, Updated: time.Now()},
			objDescOut:  &model.DBObject{},
			objOut:      &model.SafeObject{Version: int32(1), Updated: now},
			errDescCode: lerrors.StObjectNotFound,
			errUpdCode:  -1,
			errCode:     -1,
		},
		{name: "GetObjectDesc_StInternalError",
			user:        int32(1),
			objIn:       &model.SafeObject{Id: int32(1), Version: int32(1), Deleted: false, Updated: time.Now()},
			objDescOut:  &model.DBObject{},
			objOut:      &model.SafeObject{Version: int32(1), Updated: now},
			errDescCode: lerrors.StInternalError,
			errUpdCode:  -1,
			errCode:     -1,
		},
		{name: "UpdateObject_StInternalError",
			user:        int32(1),
			objIn:       &model.SafeObject{Id: int32(1), Version: int32(1), Deleted: false, Updated: time.Now()},
			objDescOut:  &model.DBObject{Id: int32(1), User: int32(1), Version: int32(1), Deleted: false},
			objOut:      &model.SafeObject{Version: int32(1), Updated: now},
			errDescCode: -1,
			errUpdCode:  lerrors.StInternalError,
			errCode:     -1,
		},
		{name: "UpdateObject_StObjectTooOld_deleted",
			user:        int32(2),
			objIn:       &model.SafeObject{Id: int32(2), Version: int32(1), Deleted: false, Updated: time.Now()},
			objDescOut:  &model.DBObject{Id: int32(2), User: int32(2), Version: int32(1), Deleted: true},
			objOut:      &model.SafeObject{Version: int32(1), Updated: now},
			errDescCode: -1,
			errUpdCode:  -1,
			errCode:     lerrors.StObjectTooOld,
		},
		{name: "UpdateObject_StObjectTooOld_version",
			user:        int32(2),
			objIn:       &model.SafeObject{Id: int32(2), Version: int32(1), Deleted: false, Updated: time.Now()},
			objDescOut:  &model.DBObject{Id: int32(2), User: int32(2), Version: int32(2), Deleted: false},
			objOut:      &model.SafeObject{Version: int32(1), Updated: now},
			errDescCode: -1,
			errUpdCode:  -1,
			errCode:     lerrors.StObjectTooOld,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			// Создаем мок
			mockStorage := mocks.NewDataStorage(t)

			var errDesc error
			if test.errDescCode > 0 {
				errDesc = &lerrors.LError{Code: test.errDescCode}
				mockStorage.On("GetObjectDesc", mock.Anything, test.user, test.objIn.Id).Return(test.objDescOut, errDesc)
			} else {

				mockStorage.On("GetObjectDesc", mock.Anything, test.user, test.objIn.Id).Return(test.objDescOut, errDesc)

				if test.errCode <= 0 {

					var errUpd error
					if test.errUpdCode > 0 {
						errUpd = &lerrors.LError{Code: test.errUpdCode}
					}

					mockStorage.On("UpdateObject", mock.Anything, mock.Anything).Return(errUpd)
				}
			}

			service, err := NewDataService(mockStorage)
			require.NoError(t, err, "NewDataService() error")

			err = service.EditSafeObject(context.Background(), test.user, test.objIn)

			if err != nil {

				var lErr *lerrors.LError

				if test.errDescCode > 0 {
					require.ErrorAs(t, err, &lErr)
					require.Equal(t, test.errDescCode, lErr.Code)
				} else if test.errUpdCode > 0 {
					require.ErrorAs(t, err, &lErr)
					require.Equal(t, test.errUpdCode, lErr.Code)
				} else if test.errCode > 0 {
					require.ErrorAs(t, err, &lErr)
					require.Equal(t, test.errCode, lErr.Code)
				} else {
					require.NoError(t, err)
				}
			}

			require.Equal(t, test.objOut.Version, test.objIn.Version)
			require.True(t, test.objIn.Updated.After(now))

		})
	}

}

func TestDelSafeObject(t *testing.T) {

	now := time.Now()

	type testCase struct {
		name        string
		user        int32
		objIn       *model.SafeObject
		objDescOut  *model.DBObject
		errDescCode lerrors.Status
		errUpdCode  lerrors.Status
		errCode     lerrors.Status
	}

	tests := []testCase{
		{name: "success",
			user:        int32(1),
			objIn:       &model.SafeObject{Id: int32(1), Version: int32(1), Deleted: false, Updated: now},
			objDescOut:  &model.DBObject{Id: int32(1), User: int32(1), Version: int32(1), Deleted: false},
			errDescCode: -1,
			errUpdCode:  -1,
			errCode:     -1,
		},
		{name: "GetObjectDesc_StObjectNotFound",
			user:        int32(1),
			objIn:       &model.SafeObject{Id: int32(1), Version: int32(1), Deleted: false, Updated: time.Now()},
			objDescOut:  &model.DBObject{},
			errDescCode: lerrors.StObjectNotFound,
			errUpdCode:  -1,
			errCode:     -1,
		},
		{name: "GetObjectDesc_StInternalError",
			user:        int32(1),
			objIn:       &model.SafeObject{Id: int32(1), Version: int32(1), Deleted: false, Updated: time.Now()},
			objDescOut:  &model.DBObject{},
			errDescCode: lerrors.StInternalError,
			errUpdCode:  -1,
			errCode:     -1,
		},
		{name: "UpdateObject_StInternalError",
			user:        int32(1),
			objIn:       &model.SafeObject{Id: int32(1), Version: int32(1), Deleted: false, Updated: time.Now()},
			objDescOut:  &model.DBObject{Id: int32(1), User: int32(1), Version: int32(1), Deleted: false},
			errDescCode: -1,
			errUpdCode:  lerrors.StInternalError,
			errCode:     -1,
		},
		{name: "UpdateObject_StObjectTooOld_version",
			user:        int32(2),
			objIn:       &model.SafeObject{Id: int32(2), Version: int32(1), Deleted: false, Updated: time.Now()},
			objDescOut:  &model.DBObject{Id: int32(2), User: int32(2), Version: int32(2), Deleted: false},
			errDescCode: -1,
			errUpdCode:  -1,
			errCode:     lerrors.StObjectTooOld,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			// Создаем мок
			mockStorage := mocks.NewDataStorage(t)

			var errDesc error
			if test.errDescCode > 0 {
				errDesc = &lerrors.LError{Code: test.errDescCode}
				mockStorage.On("GetObjectDesc", mock.Anything, test.user, test.objIn.Id).Return(test.objDescOut, errDesc)
			} else {

				mockStorage.On("GetObjectDesc", mock.Anything, test.user, test.objIn.Id).Return(test.objDescOut, errDesc)

				if test.errCode <= 0 {

					var errUpd error
					if test.errUpdCode > 0 {
						errUpd = &lerrors.LError{Code: test.errUpdCode}
					}

					mockStorage.On("UpdateObject", mock.Anything, mock.MatchedBy(func(object *model.DBObject) bool {
						// Проверяем что объект правильно сформирован
						return object.Data == nil && object.Deleted
					})).Return(errUpd)
				}
			}

			service, err := NewDataService(mockStorage)
			require.NoError(t, err, "NewDataService() error")

			err = service.DelSafeObject(context.Background(), test.user, test.objIn.Id, test.objIn.Version)

			if err != nil {

				var lErr *lerrors.LError

				if test.errDescCode > 0 {
					require.ErrorAs(t, err, &lErr)
					require.Equal(t, test.errDescCode, lErr.Code)
				} else if test.errUpdCode > 0 {
					require.ErrorAs(t, err, &lErr)
					require.Equal(t, test.errUpdCode, lErr.Code)
				} else if test.errCode > 0 {
					require.ErrorAs(t, err, &lErr)
					require.Equal(t, test.errCode, lErr.Code)
				} else {
					require.NoError(t, err)
				}
			}

		})
	}

}

func TestGetSafeObject(t *testing.T) {

	now := time.Now()

	type testCase struct {
		name        string
		user        int32
		obj         int32
		objOut      *model.SafeObject
		objDescOut  *model.DBObject
		errDescCode lerrors.Status
		errDataCode lerrors.Status
		errCode     lerrors.Status
	}

	tests := []testCase{
		{name: "success",
			user:        int32(1),
			obj:         int32(1),
			objDescOut:  &model.DBObject{Id: 1, User: int32(1), Name: "name", Kind: "kind", Data: []byte("data"), CheckSum: "1", Client: 1, Deleted: false, Version: int32(2), Updated: now},
			objOut:      &model.SafeObject{Id: 1, Name: "name", Kind: "kind", Data: []byte("data"), CheckSum: "1", Client: 1, Deleted: false, Version: int32(2), Updated: now},
			errDescCode: -1,
			errDataCode: -1,
			errCode:     -1,
		},
		{name: "GetObjectDesc_StObjectNotFound",
			user:        int32(1),
			obj:         int32(1),
			objDescOut:  nil,
			objOut:      nil,
			errDescCode: lerrors.StObjectNotFound,
			errDataCode: -1,
			errCode:     -1,
		},
		{name: "GetObjectDesc_StInternalError",
			user:        int32(1),
			obj:         int32(1),
			objDescOut:  nil,
			objOut:      nil,
			errDescCode: lerrors.StInternalError,
			errDataCode: -1,
			errCode:     -1,
		},
		{name: "GetObjectData_StObjectNotFound",
			user:        int32(1),
			obj:         int32(1),
			objDescOut:  &model.DBObject{Id: 1, User: int32(1), Name: "name", Kind: "kind", Data: []byte("data"), CheckSum: "1", Client: 1, Deleted: false, Version: int32(2), Updated: now},
			objOut:      nil,
			errDescCode: -1,
			errDataCode: lerrors.StObjectNotFound,
			errCode:     -1,
		},
		{name: "GetObjectData_StInternalError",
			user:        int32(1),
			obj:         int32(1),
			objDescOut:  &model.DBObject{Id: 1, User: int32(1), Name: "name", Kind: "kind", Data: []byte("data"), CheckSum: "1", Client: 1, Deleted: false, Version: int32(2), Updated: now},
			objOut:      nil,
			errDescCode: -1,
			errDataCode: lerrors.StInternalError,
			errCode:     -1,
		},
		{name: "GetObjectData_Deleted",
			user:        int32(1),
			obj:         int32(1),
			objDescOut:  &model.DBObject{Id: 1, User: int32(1), Name: "name", Kind: "kind", Data: []byte("data"), CheckSum: "1", Client: 1, Deleted: true, Version: int32(2), Updated: now},
			objOut:      nil,
			errDescCode: -1,
			errDataCode: -1,
			errCode:     lerrors.StObjectNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			// Создаем мок
			mockStorage := mocks.NewDataStorage(t)

			var errDesc error
			if test.errDescCode > 0 {
				errDesc = &lerrors.LError{Code: test.errDescCode}
				mockStorage.On("GetObjectDesc", mock.Anything, test.user, test.obj).Return(test.objDescOut, errDesc)
			} else {

				mockStorage.On("GetObjectDesc", mock.Anything, test.user, test.obj).Return(test.objDescOut, errDesc)

				if test.errCode <= 0 {

					var errData error
					if test.errDataCode > 0 {
						errData = &lerrors.LError{Code: test.errDataCode}
					}

					mockStorage.On("GetObjectData", mock.Anything, test.user, test.obj).Return(test.objDescOut.Data, errData)
				}
			}

			service, err := NewDataService(mockStorage)
			require.NoError(t, err, "NewDataService() error")

			obj, err := service.GetSafeObject(context.Background(), test.user, test.obj)

			if err != nil {

				var lErr *lerrors.LError

				if test.errDescCode > 0 {
					require.ErrorAs(t, err, &lErr)
					require.Equal(t, test.errDescCode, lErr.Code)
				} else if test.errDataCode > 0 {
					require.ErrorAs(t, err, &lErr)
					require.Equal(t, test.errDataCode, lErr.Code)
				} else if test.errCode > 0 {
					require.ErrorAs(t, err, &lErr)
					require.Equal(t, test.errCode, lErr.Code)
				} else {
					require.NoError(t, err)
				}
			}

			require.Equal(t, test.objOut, obj)
			/*require.Equal(t, test.objOut.Version, obj.Version)
			require.Equal(t, test.objOut.Deleted, obj.Deleted)
			require.Equal(t, test.objOut.Updated, obj.Updated)
			require.Equal(t, test.objOut.Client, obj.Client)*/

		})
	}

}

/*
func TestDataService_GetSafeObject(t *testing.T) {
	now := time.Now()

	// Создаем мок
	mockStorage := mocks.NewDataStorage(t)

	// Настраиваем ожидания для GetObjectDesc
	mockStorage.On("GetObjectDesc", mock.Anything, int32(1), int32(123)).
		Return(&model.DBObject{
			Id:       1,
			User:     123,
			Name:     "test-object",
			Kind:     "password",
			CheckSum: "checksum123",
			Updated:  now,
			Version:  1,
			Client:   100,
			Deleted:  false,
		}, nil)

	// Настраиваем ожидания для GetObjectData
	mockStorage.On("GetObjectData", mock.Anything, int32(1), int32(123)).
		Return([]byte("encrypted-data"), nil)

	// Создаем сервис
	service, err := NewDataService(mockStorage)
	require.NoError(t, err, "NewDataService() error")

	// Выполняем тестируемую функцию
	obj, err := service.GetSafeObject(context.Background(), 123, 1)

	// Проверяем результат
	require.NoError(t, err, "GetSafeObject() error")
	require.NotNil(t, obj, "Object should not be nil")
	require.Equal(t, int32(1), obj.Id, "Object Id mismatch")
	require.Equal(t, "test-object", obj.Name, "Object Name mismatch")
	require.Equal(t, []byte("encrypted-data"), obj.Data, "Object Data mismatch")
	require.False(t, obj.Deleted, "Object Deleted should be false")

	// Проверяем, что все ожидания выполнены
	mockStorage.AssertExpectations(t)
}

func TestDataService_GetSafeObject_Deleted(t *testing.T) {
	// Создаем мок
	mockStorage := mocks.NewDataStorage(t)

	// Настраиваем ожидания - возвращаем удаленный объект
	mockStorage.On("GetObjectDesc", mock.Anything, int32(1), int32(123)).
		Return(&model.DBObject{
			Id:      1,
			User:    123,
			Name:    "deleted-object",
			Deleted: true,
		}, nil)

	// Создаем сервис
	service, err := NewDataService(mockStorage)
	require.NoError(t, err, "NewDataService() error")

	// Выполняем тестируемую функцию
	_, err = service.GetSafeObject(context.Background(), 123, 1)

	// Проверяем ошибку
	require.Error(t, err, "GetSafeObject() expected error for deleted object, got nil")

	var lErr *lerrors.LError
	require.ErrorAs(t, err, &lErr, "Error should be *lerrors.LError")
	require.Equal(t, lerrors.StObjectNotFound, lErr.Code, "Error code mismatch")
	require.Contains(t, lErr.Message, "удален", "Error message should mention deletion")

	// Проверяем, что все ожидания выполнены
	mockStorage.AssertExpectations(t)
}

func TestDataService_GetSafeObject_NotFound(t *testing.T) {
	// Создаем мок
	mockStorage := mocks.NewDataStorage(t)

	// Настраиваем ожидания - объект не найден
	expectedErr := lerrors.NewLEObjectNotFound(fmt.Errorf("object not found inside"), "object not found")
	mockStorage.On("GetObjectDesc", mock.Anything, int32(999), int32(123)).
		Return(nil, expectedErr)

	// Создаем сервис
	service, err := NewDataService(mockStorage)
	require.NoError(t, err, "NewDataService() error")

	// Выполняем тестируемую функцию
	_, err = service.GetSafeObject(context.Background(), 123, 999)

	// Проверяем ошибку
	require.Error(t, err, "GetSafeObject() expected error, got nil")

	var lErr *lerrors.LError
	require.ErrorAs(t, err, &lErr, "Error should be *lerrors.LError")
	require.Equal(t, lerrors.StObjectNotFound, lErr.Code, "Error code mismatch")

	// Проверяем, что все ожидания выполнены
	mockStorage.AssertExpectations(t)
}

*/
