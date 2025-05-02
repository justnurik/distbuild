# DistBuild - Распределённая система сборки

[![Go Report Card](https://goreportcard.com/badge/gitlab.com/justnurik/distbuild)](https://goreportcard.com/report/gitlab.com/justnurik/distbuild)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

**Система распределённой сборки с поддержкой двух режимов работы: CLI и библиотеки**


## Особенности
**Использование как CLI-утилиты или embedded-библиотеки**

## Способы использования

### 1. Как консольная утилита
```bash
# Запуск координатора с кастомным портом
./distbuild coordinator --port=9090 --log-level=debug

# Подключение воркера к кластеру
./distbuild worker \
  --coordinator=localhost:9090 \
  --work-dir=/var/distbuild

# Отправка задания на сборку
curl -X POST http://localhost:9090/build \
  -H "Content-Type: application/json" \
  -d @pipeline.json
```

### 2. Как библиотека
```go
import (
  "gitlab.com/justnurik/distbuild/pkg/dist"
  "gitlab.com/justnurik/distbuild/pkg/client"
  "gitlab.com/justnurik/distbuild/pkg/worker"
)

func main() {
  // ----------------- Инициализация координатора -----------------
  cache, _ := filecache.New("./cache")
  coord := dist.NewCoordinator(
    zap.NewProductionConfig()
    core.WithCacheDir("/opt/distbuild/cache"),
  )
  defer coordinator.Stop()

  router := http.NewServeMux()
  router.Handle("/coordinator/", http.StripPrefix("/coordinator", coordinator))

  go htpp.ListenAndServe(":8080", router)


  // ----------------- Запуск воркера -----------------
  fileCache, _ := filecache.New(a.fileCachePath)
  artifacts, _ := artifact.NewCache(a.artifactCachePath)

  endpoint := fmt.Sprintf("%s/worker/0", addr)
  worker := worker.New(api.WorkerID(endpoint), a.coordinatorEndpoint, logger, fileCache, artifacts)

  router := http.NewServeMux()
  router.Handle(fmt.Sprintf("/worker/%s/", a.id), http.StripPrefix("/worker/"+a.id, worker))

  go htpp.ListenAndServe(":6029", router)


  // ----------------- Запуск сборки -----------------
  result := client.StartBuild(context.TODO(), buildGraph)
}
```

## Архитектура системы
```
                       +---------------------+
                       |      Клиент         |
                       | (Отправка графа     |
                       |  сборки, запросы)   |
                       +---------------------+
                                  ^
                                  |
                                  | HTTP/WebSocket
                                  v
+-----------------------------------------------------------------+
|                      Координатор                                |
| +---------------------+       +---------------------+           |
| |    HTTP Сервер      |       |   Планировщик       |           |
| | (Роуты:             +<----->+ (Очередь задач,     |           |
| |  /build, /signal)   |       |  распределение      |           |
| +----------+----------+       +--------+------------+           |
|            |                           |                        |
|            |                           |                        |
| +----------v---------+   +-------------v----------------+       |
| |   Метаданные       |   |   Кэш файлов                 |       |
| | (Графы сборки,     |   | (Локальное хранилище)        |       |
| |  статусы задач)    |   |                              |       |
| +--------------------+   +------------------------------+       |
+-----------------------------------------------------------------+
                                  ^
                                  | HTTP REST API
                                  | (Задачи, статусы)
           +----------------------+----------------------+
           |                      |                      |
+----------v----------+ +---------v-----------+ +--------v-----------+
|      Воркер 1       | |      Воркер 2       | |      Воркер N      |
| +-----------------+ | | +-----------------+ | | +-----------------+|
| |  Исполнитель    | | | |  Исполнитель    | | | |  Исполнитель    ||
| | (Запуск команд, | | | | (Запуск команд, | | | | (Запуск команд, ||
| |  обработка deps)| | | |  обработка deps)| | | |  обработка deps)||
| +-------+---------+ | | +-------+---------+ | | +-------+---------+|
|         |           | |         |           | |         |          |
| +-------v---------+ | | +-------v---------+ | | +-------v---------+|
| | Локальный кэш   | | | | Локальный кэш   | | | | Локальный кэш   ||
| | (Артефакты,     | | | | (Артефакты,     | | | | (Артефакты,     ||
| | временные файлы)| | | | временные файлы)| | | | временные файлы)||
| +------------------+| | +------------------+| | +-----------------+|
+---------------------+ +---------------------+ +--------------------+
```

## Текущий статус

### Реализовано
- Базовый HTTP API для управления задачами
- Локальное кэширование артефактов
- Простейший FIFO-планировщик
- Поддержка графа зависимостей
- Логирование

### Планируемые задачи
- [ ] использование websocket-ов вместо постоянных flush-ов со стороны координатора
- [ ] написать более эффективный планировщик
- [ ] большее интеграционных тестов
- [ ] покрытие тестами >= 90%
- [ ] написание бенчмарков

## Лицензия ⚖️
Проект распространяется под лицензией [MIT](LICENSE). Коммерческое использование разрешено с указанием авторства.

---

**Автор**: Нурдаулет Молданазаров  
**Контакты**: [Telegram](https://t.me/jnurik) | [Email](mailto:moldanazarov.n@phystech.edu)