// Package storage реализует слой доступа к данным (репозиторий) для приложения SafeKeeper.
// Предоставляет реализацию хранилища на основе PostgreSQL с поддержкой миграций

package storage

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/qesterrx/SafeKeeper/cmd/server/internal/lerrors"
	"github.com/qesterrx/SafeKeeper/cmd/server/internal/model"
	"github.com/qesterrx/SafeKeeper/internal/logger"
)

// PGSQL представляет драйвер для работы с базой данных PostgreSQL
type PGSQL struct {
	db *sql.DB
}

// NewStoragePGSQL создаёт новый экземпляр хранилища PostgreSQL
// Параметры:
//   - dbDSN: строка подключения к PostgreSQL в формате DSN
//     (например, "postgres://user:pass@localhost:5432/dbname?sslmode=disable")
//
// Возвращает:
//   - *PGSQL: указатель на инициализированное хранилище
//   - error: ошибка подключения, применения миграций или инициализации
func NewStoragePGSQL(dbDSN string) (*PGSQL, error) {

	//Создаем подключение
	conn, err := sql.Open("pgx", dbDSN)
	if err != nil {
		logger.Log.Error("Ошибка подключения к БД %v", err.Error())
		return nil, err
	}

	//Проверка подключения
	if err := conn.Ping(); err != nil {
		logger.Log.Error("Ошибка при проверке соединения с БД %v", err.Error())
		return nil, err
	}

	//Миграции
	driver, err := postgres.WithInstance(conn, &postgres.Config{})
	if err != nil {
		logger.Log.Error("Ошибка при создании инстанса БД для миграций %v", err.Error())
		return nil, err
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver)
	if err != nil {
		logger.Log.Error("Ошибка при создании экземпляра миграции %v", err.Error())
		return nil, err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		logger.Log.Error("Ошибка применения миграций %v", err.Error())
		return nil, err
	}

	pg := PGSQL{db: conn}

	return &pg, nil
}

// Close закрывает соединение с базой данных.
// Должен быть вызван при завершении работы приложения для корректного освобождения ресурсов
//
// Возвращает:
//   - error: ошибка при закрытии соединения (если возникла)
func (pg *PGSQL) Close() error {
	return pg.db.Close()
}

// NewUser создаёт нового пользователя в базе данных
// Выполняет вставку записи в таблицу TUSERS
// Параметры:
//   - ctx: контекст выполнения
//   - login: логин пользователя (уникальный)
//   - password: хеш пароля (bcrypt)
//   - aestoken: зашифрованный AES-токен пользователя
//
// Возвращает:
//   - error: nil при успешном создании, иначе типизированная ошибка
//   - lerrors.ErrUserAlreadyExists: при попытке создать пользователя с существующим логином
//   - lerrors.ErrInternalError: при других ошибках БД
func (pg *PGSQL) NewUser(ctx context.Context, login, password, aestoken string) error {

	args := pgx.NamedArgs{
		"LOGIN":    login,
		"PASSWORD": password,
		"AESTOKEN": aestoken,
	}

	_, err := pg.db.Exec("INSERT INTO TUSERS (DFLOGIN, DFPASSWORD, DFAESTOKEN) VALUES (@LOGIN, @PASSWORD, @AESTOKEN)", args)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				logger.Log.Info("NewUser: Пользователь с логином %s уже зарегистрирован", login)
				return lerrors.NewLEUserAlreadyExists(login)
			}
		}

		logger.Log.Error("NewUser: %v", err.Error())
		return lerrors.NewLEInternalError(err.Error())
	}

	return nil
}

// GetUserByLogin получает пользователя из базы данных по логину
// Используется при аутентификации для проверки существования пользователя
// Параметры:
//   - ctx: контекст выполнения
//   - login: логин пользователя
//
// Возвращает:
//   - *model.DBUser: указатель на модель пользователя (содержит ID, логин, хеш пароля, AES-токен)
//   - error: ошибка при поиске
//   - lerrors.ErrUserWrongPassword: если пользователь не найден (для единообразия с ошибкой пароля)
//   - lerrors.ErrInternalError: при других ошибках БД
func (pg *PGSQL) GetUserByLogin(ctx context.Context, login string) (*model.DBUser, error) {

	args := pgx.NamedArgs{
		"LOGIN": login,
	}

	user := model.DBUser{}

	err := pg.db.QueryRow("SELECT DFUSER, DFLOGIN, DFPASSWORD, DFAESTOKEN FROM TUSERS WHERE DFLOGIN=@LOGIN", args).
		Scan(&user.ID, &user.Login, &user.Password, &user.AESToken)

	if err != nil {
		if err == sql.ErrNoRows {
			logger.Log.Info("GetUser: пользователь с логином %s не найден", login)
			return nil, lerrors.NewLEUserWrongPassword(login)
		}

		logger.Log.Error("GetUser: %v", err.Error())
		return nil, lerrors.NewLEInternalError(err.Error())
	}

	return &user, nil
}

// GetUserById получает пользователя из базы данных по идентификатору
// Параметры:
//   - ctx: контекст выполнения
//   - id: идентификатор пользователя
//
// Возвращает:
//   - *model.DBUser: указатель на модель пользователя
//   - error: ошибка при поиске
//   - lerrors.ErrUserWrongPassword: если пользователь не найден
//   - lerrors.ErrInternalError: при других ошибках БД
func (pg *PGSQL) GetUserById(ctx context.Context, id int32) (*model.DBUser, error) {

	args := pgx.NamedArgs{
		"DFUSER": id,
	}

	user := model.DBUser{}

	err := pg.db.QueryRow("SELECT DFUSER, DFLOGIN, DFPASSWORD, DFAESTOKEN FROM TUSERS WHERE DFUSER=@DFUSER", args).
		Scan(&user.ID, &user.Login, &user.Password, &user.AESToken)

	if err != nil {
		if err == sql.ErrNoRows {
			logger.Log.Info("GetUserId: пользователь с ид %d не найден", id)
			return nil, lerrors.NewLEUserWrongPassword(strconv.Itoa(int(id)))
		}

		logger.Log.Error("GetUserId: %v", err.Error())
		return nil, lerrors.NewLEInternalError(err.Error())
	}

	return &user, nil
}

// GetObjectDesc получает описание защищённого объекта (без данных)
// Параметры:
//   - ctx: контекст выполнения
//   - id: идентификатор объекта
//   - user: идентификатор пользователя-владельца
//
// Возвращает:
//   - *model.DBObject: указатель на объект с метаданными (поле Data = nil)
//   - error: ошибка при получении
//   - lerrors.ErrObjectNotFound: если объект не найден или не принадлежит пользователю
//   - lerrors.ErrInternalError: при других ошибках БД
func (pg *PGSQL) GetObjectDesc(ctx context.Context, id, user int32) (*model.DBObject, error) {

	args := pgx.NamedArgs{
		"OBJECT": id,
		"USER":   user,
	}

	obj := model.DBObject{Id: id, User: user}

	err := pg.db.QueryRow(`SELECT DFNAME, DFKIND, DFCHECKSUM, DFUPDATED, DFVERSION, DFCLIENT, DFDELETED FROM TOBJECTS WHERE DFOBJECT=@OBJECT AND DFUSER=@USER`, args).
		Scan(&obj.Name, &obj.Kind, &obj.CheckSum, &obj.Updated, &obj.Version, &obj.Client, &obj.Deleted)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Log.Info("GetObjectDesc: объект с ИД %d пользователя %d не найден", id, user)
			return nil, lerrors.NewLEObjectNotFound(strconv.Itoa(int(id)))
		}

		logger.Log.Error("GetObjectDesc: %v", err.Error())
		return nil, lerrors.NewLEInternalError(err.Error())
	}
	return &obj, nil
}

// GetObjectData получает только зашифрованные данные объекта
// Параметры:
//   - ctx: контекст выполнения
//   - id: идентификатор объекта
//   - user: идентификатор пользователя-владельца
//
// Возвращает:
//   - []byte: зашифрованные данные объекта
//   - error: ошибка при получении
//   - lerrors.ErrObjectNotFound: если объект не найден или не принадлежит пользователю
//   - lerrors.ErrInternalError: при других ошибках БД
func (pg *PGSQL) GetObjectData(ctx context.Context, id, user int32) ([]byte, error) {

	args := pgx.NamedArgs{
		"OBJECT": id,
		"USER":   user,
	}

	res := []byte{}

	err := pg.db.QueryRow(`SELECT DFDATA FROM TOBJECTS WHERE DFOBJECT=@OBJECT AND DFUSER=@USER`, args).
		Scan(&res)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Log.Info("GetObjectData: объект с ИД %d пользователя %d не найден", id, user)
			return nil, lerrors.NewLEObjectNotFound(strconv.Itoa(int(id)))
		}

		logger.Log.Error("GetObjectData: %v", err.Error())
		return nil, lerrors.NewLEInternalError(err.Error())
	}
	return res, nil
}

// GetObjectListDesc получает список описаний всех объектов пользователя
// Возвращает только метаданные без данных (Data = nil) для экономии ресурсов
// Параметры:
//   - ctx: контекст выполнения
//   - user: идентификатор пользователя-владельца
//
// Возвращает:
//   - []*model.DBObject: слайс указателей на объекты (без данных)
//   - error: ошибка при выполнении запроса
//   - lerrors.ErrInternalError: при ошибках БД
func (pg *PGSQL) GetObjectListDesc(ctx context.Context, user int32) ([]*model.DBObject, error) {

	args := pgx.NamedArgs{
		"USER": user,
	}

	rows, err := pg.db.Query(`SELECT DFOBJECT, DFNAME, DFKIND, DFCHECKSUM, DFUPDATED, DFVERSION, DFCLIENT, DFDELETED FROM TOBJECTS WHERE DFUSER=@USER`, args)
	if err != nil {

		logger.Log.Error("GetObjectDesc: %v", err.Error())
		return nil, lerrors.NewLEInternalError(err.Error())
	}
	defer rows.Close()

	objs := []*model.DBObject{}
	for rows.Next() {
		obj := model.DBObject{User: user}
		err := rows.Scan(&obj.Id, &obj.Name, &obj.Kind, &obj.CheckSum, &obj.Updated, &obj.Version, &obj.Client, &obj.Deleted)
		if err != nil {
			logger.Log.Info("GetObjectListDesc rows: %v", err)
			return nil, lerrors.NewLEInternalError(err.Error())
		}

		objs = append(objs, &obj)
	}

	if rows.Err() != nil {
		logger.Log.Info("GetObjectListDesc after rows: %v", rows.Err())
		return nil, lerrors.NewLEInternalError(rows.Err().Error())
	}

	return objs, nil
}

// NewObject создаёт новый защищённый объект в базе данных
// Параметры:
//   - ctx: контекст выполнения
//   - object: указатель на DBObject для сохранения (должен содержать все поля)
//
// Возвращает:
//   - int32: сгенерированный идентификатор объекта
//   - error: ошибка при вставке
//   - lerrors.ErrInternalError: при ошибках БД
func (pg *PGSQL) NewObject(ctx context.Context, object *model.DBObject) (int32, error) {

	var id int32

	args := pgx.NamedArgs{
		"USER":     object.User,
		"NAME":     object.Name,
		"KIND":     object.Kind,
		"DATA":     object.Data,
		"CHECKSUM": object.CheckSum,
		"UPDATED":  object.Updated,
		"VERSION":  object.Version,
		"CLIENT":   object.Client,
		"DELETED":  object.Deleted,
	}
	err := pg.db.QueryRow(
		`INSERT INTO TOBJECTS (DFUSER, DFNAME, DFKIND, DFDATA, DFCHECKSUM, DFUPDATED, DFVERSION, DFCLIENT, DFDELETED)
		VALUES (@USER, @NAME, @KIND, @DATA, @CHECKSUM, @UPDATED, @VERSION, @CLIENT, @DELETED)
		RETURNING DFOBJECT`, args).Scan(&id)
	if err != nil {
		logger.Log.Error("NewObject: %v", err.Error())
		return 0, lerrors.NewLEInternalError(err.Error())
	}

	return id, nil
}

// UpdateObject обновляет существующий защищённый объект в базе данных
// Параметры:
//   - ctx: контекст выполнения
//   - object: указатель на DBObject с обновлёнными данными (должен содержать ID)
//
// Возвращает:
//   - error: nil при успешном обновлении
//   - lerrors.ErrInternalError: при ошибках БД
func (pg *PGSQL) UpdateObject(ctx context.Context, object *model.DBObject) error {

	args := pgx.NamedArgs{
		"OBJECT":   object.Id,
		"NAME":     object.Name,
		"KIND":     object.Kind,
		"DATA":     object.Data,
		"CHECKSUM": object.CheckSum,
		"UPDATED":  object.Updated,
		"VERSION":  object.Version,
		"CLIENT":   object.Client,
		"DELETED":  object.Deleted,
	}

	_, err := pg.db.Exec(
		`UPDATE TOBJECTS SET
		   DFNAME=@NAME,
		   DFKIND=@KIND,
		   DFDATA=@DATA,
		   DFCHECKSUM=@CHECKSUM,
		   DFUPDATED=@UPDATED,
		   DFVERSION=@VERSION,
		   DFCLIENT=@CLIENT,
		   DFDELETED=@DELETED
		 WHERE DFOBJECT=@OBJECT`, args)
	if err != nil {
		logger.Log.Error("UpdateObject: %v", err.Error())
		return lerrors.NewLEInternalError(err.Error())
	}

	return nil
}
