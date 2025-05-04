# filecache

Пакет `filecache` занимается хранением кеша файлов и определяет протокол передачи файлов между частями системы.

`filecache.Cache` управляет файлами и занимается контролем одновременного доступа.

## Передача файлов

Тип `filecache.Handler` реализует handler, позволяющий заливать и скачивать файлы из кеша.

- Вызов `GET /file?id=123` возвращает содержимое файла с `id=123`.
- Вызов `PUT /file?id=123` заливает содержимое файла с `id=123`

## Примеры

Инициализация
```go
cache := filecache.New("/storage/path")
handler := filecache.NewHandler(logger, cache)
client := filecache.NewClient(logger, "localhost:8080")
```

Пример записи
```go
w, abort, err := cache.Write(id)
if err != nil { ... }
defer w.Close()

// Запись данных в io.WriterCloser
_, _ = w.Write([]byte("data"))
```

Пример скачивания
```go
err := client.Download(ctx, localCache, fileID)
err := client.Upload(ctx, fileID, localPath)
```
