// Package transport реализует транспортный слой (gRPC-серверы) приложения SafeKeeper
// Обеспечивает преобразование между gRPC-запросами/ответами и внутренними сервисами приложения

package transport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/qesterrx/SafeKeeper/cmd/server/internal/model"
	"github.com/qesterrx/SafeKeeper/internal/logger"

	"github.com/qesterrx/SafeKeeper/proto/data"
	pb "github.com/qesterrx/SafeKeeper/proto/data"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// DataService определяет интерфейс сервиса управления данными, используемый транспортным слоем
// Содержит бизнес-логику CRUD-операций с защищёнными объектами

type DataService interface {
	// NewSafeObject создаёт новый защищённый объект
	NewSafeObject(ctx context.Context, user int32, obj *model.SafeObject) error

	// DelSafeObject удаляет защищённый объект
	DelSafeObject(ctx context.Context, user int32, objId int32, objVersion int32) error

	// EditSafeObject редактирует существующий защищённый объект
	EditSafeObject(ctx context.Context, user int32, obj *model.SafeObject) error

	// GetSafeObject получает защищённый объект по идентификатору
	GetSafeObject(ctx context.Context, user int32, objId int32) (*model.SafeObject, error)

	// GetSafeObjectList получает список всех защищённых объектов пользователя
	GetSafeObjectList(ctx context.Context, user int32) ([]*model.SafeObject, error)
}

// SafeDataServer реализует gRPC-сервер для потокового сервиса управления данными (data.v1.SafeDataService)
type SafeDataServer struct {
	pb.UnimplementedSafeDataServiceServer
	data DataService
}

// NewSafeDataServer создаёт новый экземпляр SafeDataServer
// Принимает реализацию DataService для обработки бизнес-логики
// Параметры:
//   - dataService: реализация интерфейса DataService
//
// Возвращает:
//   - *SafeDataServer: указатель на созданный gRPC-сервер
func NewSafeDataServer(dataService DataService) *SafeDataServer {
	return &SafeDataServer{data: dataService}
}

// NewSafeData обрабатывает потоковый gRPC-запрос на создание нового защищённого объекта
// Поддерживает передачу данных по частям (чанкам) для эффективной работы с большими объектами
//
// Формат запроса:
//   - Первое сообщение должно содержать метаданные (SafeDataMeta)
//   - Последующие сообщения содержат чанки данных (SafeDataChunk)
//   - Завершение потока (EOF) сигнализирует об окончании передачи
//
// Параметры:
//   - stream: потоковый gRPC-канал для приёма запросов и отправки ответа
//
// Возвращает:
//   - error: gRPC-статус с кодом ошибки (или nil при успехе)
func (sds *SafeDataServer) NewSafeData(stream pb.SafeDataService_NewSafeDataServer) error {

	ctx := stream.Context()

	user, ok := ctx.Value("user_id").(int32)
	if !ok {
		return status.Error(codes.Internal, "пользователь не передан в контексте")
	}

	var meta *pb.SafeDataMeta
	var chunks [][]byte
	var totalSize int

	// Прием данных из стрима
	for {

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "ошибка приема данных: %v", err)
		}

		switch req.WhichRequestType() {
		case pb.NewSafeDataRequest_Meta_case:
			// Получение метаданных
			meta = req.GetMeta()
		case pb.NewSafeDataRequest_Chunk_case:
			// Получение чанка данных
			chunk := req.GetChunk()
			chunks = append(chunks, chunk.GetData())
			totalSize += len(chunk.GetData())

		default:
			return status.Errorf(codes.InvalidArgument, "неизвестный тип запроса")
		}
	}

	// Проверка наличия метаданных
	if meta == nil {
		return status.Error(codes.InvalidArgument, "метаданные не переданы")
	}

	logger.Log.Debug("Получили %d байт", totalSize)

	// Сохранение объекта
	sobj := model.SafeObject{
		Name:     meta.GetName(),
		Kind:     meta.GetKind(),
		CheckSum: meta.GetChecksum(),
		Client:   meta.GetClient(),
		Data:     make([]byte, 0, totalSize),
	}

	// Сборка данных из чанков
	for _, chunk := range chunks {
		sobj.Data = append(sobj.Data, chunk...)
	}

	err := sds.data.NewSafeObject(ctx, user, &sobj)

	if err != nil {
		return TranslateError(err)
	}

	// Отправка ответа
	var res pb.NewSafeDataResponse
	res.SetId(sobj.Id)
	res.SetVersion(sobj.Version)
	res.SetUpdated(timestamppb.New(sobj.Updated))

	return stream.SendAndClose(&res)
}

// EditSafeData обрабатывает потоковый gRPC-запрос на редактирование существующего защищённого объекта
// Поддерживает передачу данных по частям (чанкам)
//
// Формат запроса:
//   - Первое сообщение должно содержать метаданные (SafeDataMeta с ID и версией)
//   - Последующие сообщения содержат чанки данных (SafeDataChunk)
//   - Завершение потока (EOF) сигнализирует об окончании передачи
//
// Параметры:
//   - stream: потоковый gRPC-канал для приёма запросов и отправки ответа
//
// Возвращает:
//   - error: gRPC-статус с кодом ошибки (или nil при успехе)
func (sds *SafeDataServer) EditSafeData(stream pb.SafeDataService_EditSafeDataServer) error {

	ctx := stream.Context()

	user, ok := ctx.Value("user_id").(int32)
	if !ok {
		return status.Error(codes.Internal, "пользователь не передан в контексте")
	}

	var meta *pb.SafeDataMeta
	var chunks [][]byte
	var totalSize int

	// Прием данных из стрима
	for {

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "ошибка приема данных: %v", err)
		}

		switch req.WhichRequestType() {
		case pb.EditSafeDataRequest_Meta_case:
			// Получение метаданных
			meta = req.GetMeta()
		case pb.EditSafeDataRequest_Chunk_case:
			// Получение чанка данных
			chunk := req.GetChunk()
			chunks = append(chunks, chunk.GetData())
			totalSize += len(chunk.GetData())

		default:
			return status.Errorf(codes.InvalidArgument, "неизвестный тип запроса")
		}
	}

	// Проверка наличия метаданных
	if meta == nil {
		return status.Error(codes.InvalidArgument, "метаданные не переданы")
	}

	logger.Log.Debug("Получили %d байт", totalSize)

	// Сохранение объекта
	sobj := model.SafeObject{
		Id:       meta.GetId(),
		Name:     meta.GetName(),
		Kind:     meta.GetKind(),
		Version:  meta.GetVersion(),
		CheckSum: meta.GetChecksum(),
		Client:   meta.GetClient(),
	}

	// Сборка данных из чанков
	sobj.Data = make([]byte, 0, totalSize)
	for _, chunk := range chunks {
		sobj.Data = append(sobj.Data, chunk...)
	}

	err := sds.data.EditSafeObject(ctx, user, &sobj)

	if err != nil {
		return TranslateError(err)
	}

	// Отправка ответа
	var res pb.EditSafeDataResponse
	res.SetVersion(sobj.Version)
	res.SetUpdated(timestamppb.New(sobj.Updated))

	return stream.SendAndClose(&res)
}

// DelSafeData обрабатывает gRPC-запрос на удаление защищённого объекта
// Извлекает идентификатор пользователя из контекста, вызывает сервисный слой для установки флага Deleted = true
//
// Параметры:
//   - ctx: контекст gRPC-запроса (содержит user_id)
//   - req: запрос на удаление объекта (содержит ID и версию для проверки)
//
// Возвращает:
//   - *emptypb.Empty: пустой ответ при успехе
//   - error: gRPC-статус с кодом ошибки (или nil при успехе)
func (sds *SafeDataServer) DelSafeData(ctx context.Context, req *pb.DelSafeDataRequest) (*emptypb.Empty, error) {

	user, ok := ctx.Value("user_id").(int32)
	if !ok {
		return nil, status.Error(codes.Internal, "пользователь не передан в контексте")
	}

	objId := req.GetId()
	objVersion := req.GetVersion()

	err := sds.data.DelSafeObject(ctx, user, objId, objVersion)

	if err != nil {
		return nil, TranslateError(err)
	}

	return &emptypb.Empty{}, nil
}

// GetSafeDataList обрабатывает gRPC-запрос на получение списка всех защищённых объектов пользователя
// Возвращает только метаданные объектов без данных
//
// Параметры:
//   - ctx: контекст gRPC-запроса (содержит user_id)
//   - req: пустой запрос
//
// Возвращает:
//   - *pb.GetSafeDataListResponse: ответ со списком метаданных объектов
//   - error: gRPC-статус с кодом ошибки (или nil при успехе)
func (sds *SafeDataServer) GetSafeDataList(ctx context.Context, req *emptypb.Empty) (*pb.GetSafeDataListResponse, error) {

	user, ok := ctx.Value("user_id").(int32)
	if !ok {
		return nil, status.Error(codes.Internal, "пользователь не передан в контексте")
	}

	objs, err := sds.data.GetSafeObjectList(ctx, user)
	if err != nil {
		return nil, TranslateError(err)
	}

	var resObjs []*pb.SafeDataMeta

	for _, obj := range objs {

		meta := pb.SafeDataMeta{}

		meta.SetId(obj.Id)
		meta.SetName(obj.Name)
		meta.SetKind(obj.Kind)
		meta.SetVersion(obj.Version)
		meta.SetChecksum(obj.CheckSum)
		meta.SetUpdated(timestamppb.New(obj.Updated))
		meta.SetClient(obj.Client)
		meta.SetDeleted(obj.Deleted)

		resObjs = append(resObjs, &meta)
	}

	res := pb.GetSafeDataListResponse{}
	res.SetSobjects(resObjs)

	return &res, nil

}

// GetSafeData обрабатывает потоковый gRPC-запрос на получение защищённого объекта
// Отправляет данные по частям (чанкам) для эффективной передачи больших объектов
//
// Параметры:
//   - req: запрос на получение объекта (содержит ID объекта)
//   - stream: потоковый gRPC-канал для отправки ответа
//
// Возвращает:
//   - error: gRPC-статус с кодом ошибки (или nil при успехе)
func (sds *SafeDataServer) GetSafeData(req *pb.GetSafeDataRequest, stream pb.SafeDataService_GetSafeDataServer) error {

	ctx := stream.Context()

	user, ok := ctx.Value("user_id").(int32)
	if !ok {
		return status.Error(codes.Internal, "пользователь не передан в контексте")
	}

	id := req.GetId()

	obj, err := sds.data.GetSafeObject(ctx, user, id)
	if err != nil {
		return TranslateError(err)
	}

	// 1. Отправляем метаданные
	meta := pb.SafeDataMeta_builder{
		Id:       proto.Int32(obj.Id),
		Name:     proto.String(obj.Name),
		Kind:     proto.String(obj.Kind),
		Version:  proto.Int32(obj.Version),
		Updated:  timestamppb.New(obj.Updated),
		Checksum: proto.String(obj.CheckSum),
	}

	res := &data.GetSafeDataResponse{}
	res.SetMeta(meta.Build())

	if err := stream.Send(res); err != nil {

		logger.Log.ForwardError(err)
		return fmt.Errorf("ошибка отправки метаданных: %w", err)
	}

	// 2. Отправляем данные по частям (чанкам)
	databuf := bytes.NewReader(obj.Data)

	offset := 0
	buf := make([]byte, 32*1024)
	for {
		n, err := databuf.Read(buf)

		if n > 0 {

			chunk := pb.SafeDataChunk_builder{
				Data: buf[:n],
			}

			chunkReq := &data.GetSafeDataResponse{}
			chunkReq.SetChunk(chunk.Build())

			if err := stream.Send(chunkReq); err != nil {
				return fmt.Errorf("ошибка отправки чанка: %w", err)
			}

			offset = offset + n

			time.Sleep(10 * time.Millisecond)
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	logger.Log.Debug("Отправили %d байт", offset)

	return nil
}
