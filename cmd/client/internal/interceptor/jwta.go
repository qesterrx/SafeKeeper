// Package interceptor предоставляет gRPC интерцепторы для добавления JWT аутентификации в клиентские вызовы
// Пакет включает клиентские перехватчики для унарных и стриминговых RPC вызовов

package interceptor

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// JWTClientInterceptor реализует gRPC клиентские интерцепторы для автоматического добавления JWT токена аутентификации в заголовки каждого исходящего запроса
// Токен передается в метаданных в формате HTTP заголовка Authorization
type JWTClientInterceptor struct {
	jwtToken string
}

// NewJWTClientInterceptor создает новый экземпляр JWTClientInterceptor с указанным JWT токеном
func NewJWTClientInterceptor(jwtToken string) *JWTClientInterceptor {
	return &JWTClientInterceptor{
		jwtToken: jwtToken,
	}
}

// Unary возвращает унарный клиентский интерцептор, который добавляет JWT токен в метаданные исходящего контекста перед выполнением запроса
// Интерцептор автоматически встраивает заголовок Authorization в формате"Bearer <token>" в gRPC метаданные для всех унарных RPC вызовов
// Возвращает:
//   - grpc.UnaryClientInterceptor: функция-перехватчик для унарных вызовов
func (j *JWTClientInterceptor) Unary() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		// Добавляем токен в метаданные
		md := metadata.New(map[string]string{
			"authorization": "Bearer " + j.jwtToken,
		})
		ctx = metadata.NewOutgoingContext(ctx, md)

		// Выполняем запрос
		err := invoker(ctx, method, req, reply, cc, opts...)

		return err
	}
}

// Stream возвращает стриминговый клиентский интерцептор, который добавляет JWT токен в метаданные исходящего контекста перед созданием стрима
// Интерцептор автоматически встраивает заголовок Authorization в формате "Bearer <token>" в gRPC метаданные для всех стриминговых RPC вызовов
// Возвращает:
//   - grpc.StreamClientInterceptor: функция-перехватчик для стриминговых вызовов.
func (j *JWTClientInterceptor) Stream() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc,
		cc *grpc.ClientConn, method string,
		streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {

		// Добавляем токен в метаданные
		md := metadata.New(map[string]string{
			"authorization": "Bearer " + j.jwtToken,
		})
		ctx = metadata.NewOutgoingContext(ctx, md)

		// Создаем стрим
		stream, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			return nil, err
		}

		return stream, err

	}
}
