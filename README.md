# SafeKeeper
Сетевое хранилище зашифрованных данных

Тестовая подготовка Postres
```sql
create user safekeeper with password 'safekeeper'
create database safekeeper owner safekeeper
grant all privileges on database safekeeper to safekeeper;
```

Тестовая подготовка сертификатов
```bash
# Генерация CA ключа и сертификата
openssl genrsa -out ca-key.pem 4096
openssl req -new -x509 -days 365 -key ca-key.pem -out ca-cert.pem -subj "/C=RU/ST=Moscow/L=Moscow/O=MyOrg/CN=MyCA"


# Сертификат сервера
openssl genrsa -out server-key.pem 4096
openssl req -new -key server-key.pem -out server-csr.pem -config server_excample.cnf
openssl x509 -req -days 365 -in server-csr.pem -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem -extensions req_ext -extfile server_excample.cnf
```

Пример запуска сервера
```bash
go run ./cmd/server/main.go -d=postgres://safekeeper:safekeeper@localhost:5432/safekeeper?sslmode=disable -l=DEBUG -j=JWT_SECRET_KEY
```

Пример запуска клиента
```bash
go run -ldflags "-X 'main.buildClientVersion=1' -X 'main.buildInfo=Тестовая версия'" ./cmd/client/main.go -f=client.log -l=DEBUG
```
