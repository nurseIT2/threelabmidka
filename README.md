# File Storage Service

Сервис для загрузки, хранения и управления файлами с использованием MinIO и PostgreSQL.

## Технологии

- **Go** - основной язык программирования
- **PostgreSQL** - база данных для метаданных файлов
- **MinIO** - объектное хранилище для файлов
- **Docker Compose** - для запуска сервисов

## Структура проекта

- `main.go` - основной код приложения
- `docker-compose.yml` - конфигурация сервисов
- `go.mod`, `go.sum` - зависимости Go

## API Endpoints

### Upload файла
```
POST /upload
Content-Type: multipart/form-data
Body: file (form-data)
```

### Список файлов
```
GET /files
```

### Скачать файл
```
GET /download?id=<file-id>
```

### Удалить файл
```
DELETE /delete?name=<file-name>
```

## Запуск проекта

1. Запустите Docker сервисы:
```bash
docker-compose up -d
```

2. Запустите приложение:
```bash
go run main.go
```

3. Сервис будет доступен на:
- **API**: http://localhost:8080
- **MinIO Console**: http://localhost:9001
- **PostgreSQL**: localhost:5432

## Учетные данные

### PostgreSQL
- User: `postgres`
- Password: `postgres`
- Database: `filedb`

### MinIO
- Access Key: `minioadmin`
- Secret Key: `minioadmin`
- Bucket: `files`

## Тестирование в Postman

Используйте коллекцию Postman для тестирования всех CRUD операций. См. инструкцию в документации.
# threelabmidka
