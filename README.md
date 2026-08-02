# SafeKeeper
Сетевое хранилище зашифрованных данных

Тестовая подготовка Postres
```sql
create user safekeeper with password 'safekeeper'
create database safekeeper owner safekeeper
grant all privileges on database safekeeper to safekeeper;
```

Пример запуска сервера
```bash
go run ./cmd/server/main.go -d=postgres://safekeeper:safekeeper@localhost:5432/safekeeper?sslmode=disable -l=DEBUG -j=JWT_SECRET_KEY
```

Пример запуска клиента
```bash
go run -ldflags "-X 'main.buildClientVersion=1' -X 'main.buildInfo=Тестовая версия'" ./cmd/client/main.go -f=client.log -l=DEBUG
```

------------------------------------------------------------------------------------------
Много чего сделано не так как изначально задумывалось но уже выхожу из всех возможных сроков (

TUI меня мягко говоря убил в самом начале, никак не получалось у меня нормально связать его с сервисным слоем, да и текущая реализация мне не нравится
