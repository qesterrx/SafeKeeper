package transport

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"google.golang.org/grpc/credentials"

	_ "embed"
)

//go:embed ca-cert.pem
var caCert []byte // Встраиваем CA-сертификат в бинарник

func getCredentials() (*credentials.TransportCredentials, error) {
	// 1. Создаём пул доверенных корневых сертификатов
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("Не удалось добавить CA в пул")
	}

	// 3. Настраиваем TLS (клиентский сертификат НЕ предоставляем)
	tlsConfig := &tls.Config{
		RootCAs:            certPool,    // Доверяем только нашему CA
		ServerName:         "localhost", // Совпадает с CN в сертификате сервера
		InsecureSkipVerify: false,       // ВАЖНО: проверка обязательна
		MinVersion:         tls.VersionTLS13,
	}

	creds := credentials.NewTLS(tlsConfig)

	return &creds, nil
}
