# DistBuild - Распределённая система сборки

[![Go Report Card](https://goreportcard.com/badge/gitlab.com/justnurik/distbuild)](https://goreportcard.com/report/gitlab.com/justnurik/distbuild)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

**Система распределённой сборки с поддержкой двух режимов работы: CLI и библиотеки**

## Особенности
**Использование как CLI-утилиты или embedded-библиотеки**

## Быстрый старт
надо прочитать `README.md` пакета `build`, после уже прочиать `Способы использования` ниже

## Способы использования

### 1. Как консольная утилита (сложно, так как придется разбираться как работает скачивание файлов на координатора)

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

# скачивание необходимых файлов на координатора
TODO

# Сигнал координатору (сигнал о том что все необходимые файлы скачаны на координатора)
curl -X POST http://localhost:9090/signal?build_id=12345
```

### 2. Как библиотека

#### 2.1 Только клиентская часть (оптимально)

```bash
# Запуск координатора
./distbuild coordinator --port=9090 --log-level=error

# Подключение воркера к кластеру
./distbuild worker \
  --coordinator=localhost:9090 \
```

```go
package main

import (
  "gitlab.com/justnurik/distbuild/pkg/api"
  "gitlab.com/justnurik/distbuild/pkg/client"
)

func main() {
  client := client.NewClient(zap.NewProductionConfig(), "localhost:9090", ".")
  
  buildReq := api.BuildRequest{
    Jobs: []api.Job{
      {
        Name:   "test",
        Cmds:   []string{"go test ./..."},
        Inputs: []string{"*.go"},
      },
    },
  }

  // тут lsn - реализация интерфейса BuildListner (смотреть пакет client)
  if err := client.Build(context.Background(), buildReq, &lsn{}); err != nil {
    log.Fatal(err)
  }
}
```

#### 2.2 Вся система (тоже не плохо, только придется понять api соответсвующих паккетов)

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

  go htpp.ListenAndServe(":8080", router) // плохо так делать, но как пример сойдет 


  // ----------------- Запуск воркера -----------------
  fileCache, _ := filecache.New(a.fileCachePath)
  artifacts, _ := artifact.NewCache(a.artifactCachePath)

  endpoint := fmt.Sprintf("%s/worker/0", addr)
  worker := worker.New(api.WorkerID(endpoint), "localhost:9090", zap.NewProductionConfig(), fileCache, artifacts)

  router := http.NewServeMux()
  router.Handle(fmt.Sprintf("/worker/%s/", a.id), http.StripPrefix("/worker/"+a.id, worker))

  go htpp.ListenAndServe(":6029", router) // плохо так делать, но как пример сойдет 


  // ----------------- Запуск сборки -----------------
  client := client.NewClient(zap.NewProductionConfig(), "localhost:9090", ".")

  // тут lsn - реализация интерфейса BuildListner (смотреть пакет client)
  if err := client.Build(context.TODO(), buildGraph, &lsn{}); err != nil {
    log.Fatal(err)
  }
}
```

## Текущий статус

### Реализовано
- Базовый HTTP API для управления задачами
- Локальное кэширование артефактов
- Простейший FIFO-планировщик
- Поддержка графа зависимостей
- Логирование

### Планируемые задачи
- [ ] Переход на WebSocket для /build эндпоинта
- [ ] написать более эффективный планировщик
- [ ] большее интеграционных тестов
- [ ] покрытие тестами >= 90%
- [ ] написание бенчмарков

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

## Лицензия
Проект распространяется под лицензией [MIT](LICENSE). 

---

**Автор**: Нурдаулет Молданазаров  
**Контакты**: [Telegram](https://t.me/jnurik) | [Email](mailto:moldanazarov.n@phystech.edu)
