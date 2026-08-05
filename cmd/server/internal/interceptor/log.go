// Package interceptor предоставляет gRPC-перехватчики (interceptors)
// Реализует сквозную функциональность: структурированное логирование
package interceptor

import (
	"context"
	"time"

	"github.com/qesterrx/SafeKeeper/internal/logger"
	"google.golang.org/grpc"
)

// UnaryLogInterceptor возвращает унарный перехватчик для логирования всех входящих gRPC-вызовов
// Замеряет время выполнения каждого запроса и логирует информацию о методе, длительности и статусе выполнения
//
// Формат логирования:
//   - Имя: "GRPCStream" (унифицировано для всех типов вызовов)
//   - Метод: полное имя gRPC-метода (например, "/auth.v1.AuthService/Login")
//   - Длительность: время выполнения в миллисекундах
//   - Статус: "OK" при успешном выполнении или текст ошибки в случае неудачи
//
// Важно: перехватчик не влияет на выполнение запроса и не изменяет контекст или ответ
func UnaryLogInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

	start := time.Now()

	res, err := handler(ctx, req)

	status := "OK"
	if err != nil {
		status = err.Error()
	}

	duration := time.Since(start)

	logger.Log.Info("%-*s %-*s %*dms %s", 10, "GRPCStream", 30, info.FullMethod, 6, duration.Milliseconds(), status)

	return res, err

}

// StreamLogInterceptor возвращает потоковый перехватчик для логирования всех входящих
// потоковых gRPC-вызовов. Замеряет общее время выполнения потока от начала до завершения
// и логирует информацию о методе, длительности и статусе выполнения
//
// Формат логирования идентичен унарному перехватчику:
//   - Имя: "GRPCStream"
//   - Метод: полное имя gRPC-метода
//   - Длительность: общее время выполнения в миллисекундах
//   - Статус: "OK" при успешном выполнении или текст ошибки
//
// Перехватчик логирует только общее время выполнения потока, а не время каждого отдельного сообщения
func StreamLogInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {

	start := time.Now()

	err := handler(srv, ss)

	status := "OK"
	if err != nil {
		status = err.Error()
	}

	duration := time.Since(start)

	logger.Log.Info("%-*s %-*s %*dms %s", 10, "GRPCStream", 30, info.FullMethod, 6, duration.Milliseconds(), status)

	return err
}
