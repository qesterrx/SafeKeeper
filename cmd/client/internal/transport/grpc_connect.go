// Package transport предоставляет реализацию транспортного уровня для взаимодействия с gRPC-сервером
package transport

import (
	"context"
	"fmt"
	"io"

	"github.com/qesterrx/SafeKeeper/cmd/client/internal/interceptor"
	"github.com/qesterrx/SafeKeeper/cmd/client/internal/model"
	"github.com/qesterrx/SafeKeeper/internal/logger"
	"github.com/qesterrx/SafeKeeper/proto/data"
	pb "github.com/qesterrx/SafeKeeper/proto/data"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// GRPCConnect представляет gRPC-соединение с сервером для выполнения операций с объектами SafeData
// Обеспечивает потоковую передачу данных и обработку ошибок
type GRPCConnect struct {
	conn   *grpc.ClientConn
	client pb.SafeDataServiceClient

	clientVer int32
}

// NewGRPCConnect создает новое gRPC-соединение с сервером.
// Настраивает JWT-интерцепторы для аутентификации всех запросов
func NewGRPCConnect(addr, jwt string, clientVer int32) (*GRPCConnect, error) {

	grpcc := GRPCConnect{}

	//Сертификат
	cred, err := getCredentials()
	if err != nil {
		return nil, fmt.Errorf("ошибка соединения с сервером: %w", err)
	}

	//JWTA
	jwtInterceptor := interceptor.NewJWTClientInterceptor(jwt)

	//Клиент
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(*cred),
		grpc.WithUnaryInterceptor(jwtInterceptor.Unary()),
		grpc.WithStreamInterceptor(jwtInterceptor.Stream()),
	)
	if err != nil {
		return nil, fmt.Errorf("Ошибка соединения с сервером: %w", err)
	}

	client := pb.NewSafeDataServiceClient(conn)

	grpcc.conn = conn
	grpcc.client = client

	return &grpcc, nil

}

// Close закрывает gRPC-соединение с сервером
func (grpcc *GRPCConnect) Close() {
	grpcc.conn.Close()
}

// NewSafeData создает новый объект SafeData на сервере.
// Отправляет метаданные и данные объекта по частям (чанкам) через стрим
func (grpcc *GRPCConnect) NewSafeData(ctx context.Context, obj *model.TransportObject, setProcessInfo func(string)) error {
	// Создаем стрим
	stream, err := grpcc.client.NewSafeData(ctx)
	if err != nil {
		return fmt.Errorf("ошибка создания стрима: %w", err)
	}

	// 1. Отправляем метаданные
	meta := pb.SafeDataMeta_builder{
		Name:     proto.String(obj.Name),
		Kind:     proto.String(obj.Kind),
		Checksum: proto.String(obj.CheckSum),
		Client:   proto.Int32(grpcc.clientVer),
	}

	req := &data.NewSafeDataRequest{}
	req.SetMeta(meta.Build())

	if err := stream.Send(req); err != nil {

		logger.Log.ForwardError(err)
		return fmt.Errorf("ошибка отправки метаданных: %w", err)
	}

	// 2. Отправляем данные чанками

	len := obj.DataSize
	offset := 0
	buf := make([]byte, 32*1024)
	for {
		n, err := obj.Data.Read(buf)
		if n > 0 {

			chunk := pb.SafeDataChunk_builder{
				Data: buf[:n],
			}

			chunkReq := &data.NewSafeDataRequest{}
			chunkReq.SetChunk(chunk.Build())

			if err := stream.Send(chunkReq); err != nil {
				return fmt.Errorf("ошибка отправки чанка: %w", err)
			}

			offset = offset + n
			setProcessInfo(fmt.Sprintf("Отправка %d, %d, %.2f", offset, len, float32(offset)/float32(len)*100))

		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// 3. Закрываем стрим и получаем ответ
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("ошибка получения ответа: %w", err)
	}

	obj.Id = resp.GetId()
	obj.Version = resp.GetVersion()
	obj.Updated = resp.GetUpdated().AsTime()

	return nil
}

// EditSafeData редактирует существующий объект SafeData на сервере
// Отправляет обновленные метаданные и данные объекта по частям через стрим
func (grpcc *GRPCConnect) EditSafeData(ctx context.Context, obj *model.TransportObject, setProcessInfo func(string)) error {
	// Создаем стрим
	stream, err := grpcc.client.EditSafeData(ctx)
	if err != nil {
		return fmt.Errorf("ошибка создания стрима: %w", err)
	}

	// 1. Отправляем метаданные
	meta := pb.SafeDataMeta_builder{
		Id:       proto.Int32(obj.Id),
		Name:     proto.String(obj.Name),
		Kind:     proto.String(obj.Kind),
		Version:  proto.Int32(obj.Version),
		Checksum: proto.String(obj.CheckSum),
		Client:   proto.Int32(grpcc.clientVer),
	}

	req := &data.EditSafeDataRequest{}
	req.SetMeta(meta.Build())

	if err := stream.Send(req); err != nil {

		logger.Log.ForwardError(err)
		return fmt.Errorf("ошибка отправки метаданных: %w", err)
	}

	// 2. Отправляем данные чанками

	len := obj.DataSize
	offset := 0
	buf := make([]byte, 32*1024)
	for {
		n, err := obj.Data.Read(buf)
		if n > 0 {

			chunk := pb.SafeDataChunk_builder{
				Data: buf[:n],
			}

			chunkReq := &data.EditSafeDataRequest{}
			chunkReq.SetChunk(chunk.Build())

			if err := stream.Send(chunkReq); err != nil {
				return fmt.Errorf("ошибка отправки чанка: %w", err)
			}

			offset = offset + n
			setProcessInfo(fmt.Sprintf("Отправка %d, %d, %.2f", offset, len, float32(offset)/float32(len)*100))

		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// 3. Закрываем стрим и получаем ответ
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("ошибка получения ответа: %w", err)
	}

	obj.Version = resp.GetVersion()
	obj.Updated = resp.GetUpdated().AsTime()

	return nil
}

// DelSafeData удаляет объект SafeData на сервере по его ID с проверкой версии
func (grpcc *GRPCConnect) DelSafeData(ctx context.Context, obj *model.TransportObject) error {

	req := pb.DelSafeDataRequest{}
	req.SetId(obj.Id)
	req.SetVersion(obj.Version)

	_, err := grpcc.client.DelSafeData(ctx, &req)

	if err != nil {
		return err
	}

	return nil
}

// GetSafeDataList получает список всех объектов SafeData с сервера
// Возвращает только метаданные объектов без данных
func (grpcc *GRPCConnect) GetSafeDataList(ctx context.Context) ([]*model.TransportObject, error) {

	res, err := grpcc.client.GetSafeDataList(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}

	resObjs := res.GetSobjects()
	tobjs := []*model.TransportObject{}

	for _, resObj := range resObjs {
		tobj := model.TransportObject{
			Id:       resObj.GetId(),
			Name:     resObj.GetName(),
			Kind:     resObj.GetKind(),
			Version:  resObj.GetVersion(),
			Updated:  resObj.GetUpdated().AsTime(),
			Deleted:  resObj.GetDeleted(),
			CheckSum: resObj.GetChecksum(),
		}

		tobjs = append(tobjs, &tobj)
	}

	return tobjs, nil
}

// GetSafeData получает конкретный объект SafeData с сервера
// Данные передаются по частям через стрим и записываются в поле Data объекта
func (grpcc *GRPCConnect) GetSafeData(ctx context.Context, obj *model.TransportObject) error {

	req := pb.GetSafeDataRequest{}
	req.SetId(obj.Id)

	stream, err := grpcc.client.GetSafeData(ctx, &req)
	if err != nil {
		return err
	}

	size := 0
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			// Все данные получены
			break
		}
		if err != nil {
			return fmt.Errorf("failed to receive data: %v", err)
		}

		// Определяем тип полученных данных
		switch resp.WhichRequestType() {
		case pb.GetSafeDataResponse_Meta_case:
			// Получены метаданные
			meta := resp.GetMeta()
			if meta == nil {
				return fmt.Errorf("received nil meta")
			}

			obj.Version = meta.GetVersion()
			obj.CheckSum = meta.GetChecksum()
			obj.Updated = meta.GetUpdated().AsTime()
			obj.Client = meta.GetClient()
			obj.Deleted = meta.GetDeleted()

		case pb.GetSafeDataResponse_Chunk_case:
			// Получен чанк данных
			chunk := resp.GetChunk()
			if chunk == nil {
				return fmt.Errorf("received nil chunk")
			}

			data := chunk.GetData()
			n, err := obj.Data.Write(data)
			size = size + n
			if err != nil {
				return fmt.Errorf("ошибка получения данных")
			}

		default:
			return fmt.Errorf("unknown request type received")
		}
	}

	logger.Log.Debug("Получили %d байт", size)

	return nil
}

// ErrNeedReAuth проверяет, требует ли ошибка повторной аутентификации
// Анализирует gRPC-статус ошибки на наличие кода Unauthenticated
func (grpcc *GRPCConnect) ErrNeedReAuth(err error) bool {
	st, ok := status.FromError(err)
	if ok {
		if st.Code() == codes.Unauthenticated {
			return true
		}
	}
	return false
}

// ErrObjConflict проверяет, является ли ошибка конфликтом версий объекта
// Анализирует gRPC-статус ошибки на наличие кода DataLoss, который используется для обозначения конфликта версий
func (grpcc *GRPCConnect) ErrObjConflict(err error) bool {
	st, ok := status.FromError(err)
	if ok {
		if st.Code() == codes.DataLoss {
			return true
		}
	}
	return false
}
