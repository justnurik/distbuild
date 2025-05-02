# Coordinator Service

Сервис-координатор для управления распределённой сборкой проектов.

## Запуск

```bash
./coordinator \
  --port=8080 \
  --work-dir=./data \
  --log-file=logs/coordinator.log \
  --file-cache=filecache \
  --log-level=info
```

### Параметры

| Параметр         | По умолчанию     | Описание                          |
|-------------------|------------------|-----------------------------------|
| `--port`          | 8080            | Порт для HTTP-сервера             |
| `--work-dir`      | .               | Базовая директория для данных     |
| `--log-file`      | logs            | Путь к файлу логов                |
| `--file-cache`    | filecache       | Директория кэша файлов            |
| `--log-level`     | error           | Уровень логирования (debug/info/warn/error) |
