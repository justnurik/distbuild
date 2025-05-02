# Worker Service

Сервис-воркер для выполнения задач сборки.

## Запуск

```bash
./worker \
  --id=worker1 \
  --port=9090 \
  --root=./worker_data \
  --log-file=logs/worker.log \
  --file-cache=filecache \
  --artifacts=artifacts \
  --coordinator=http://localhost:8080 \
  --log-level=info
```

### Параметры

| Параметр               | По умолчанию     | Описание                          |
|------------------------|------------------|-----------------------------------|
| `--id`                 | 0               | Уникальный идентификатор воркера  |
| `--port`               | 8080            | Порт для HTTP-сервера             |
| `--root`               | .               | Корневая рабочая директория       |
| `--log-file`           | logs            | Путь к файлу логов                |
| `--file-cache`         | filecache       | Директория кэша файлов            |
| `--artifacts`          | artifacts       | Директория артефактов сборки      |
| `--coordinator`        |                 | URL сервиса-координатора          |
| `--log-level`          | error           | Уровень логирования               |

## Пример интеграции

```bash
# Запуск с подключением к координатору
./worker --coordinator=http://coordinator:8080 --id=node1
```

## Архитектура

```
[Client] -> [Coordinator] -> [Worker]
               ^     |
               |     v
           [Status Updates]
```
