# DistBuild - Распределённая система сборки

[![Go Report Card](https://goreportcard.com/badge/gitlab.com/justnurik/distbuild)](https://goreportcard.com/report/gitlab.com/justnurik/distbuild)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

**Система распределённой сборки с поддержкой двух режимов работы: CLI и библиотеки**

## Быстрый старт
Надо прочитать `README.md` пакета `build`, после уже прочиать `Способы использования` ниже

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
	logger, _ := zap.NewProduction()
	client := client.NewClient(logger, "localhost:9090", ".")

	graph := build.Graph{
		SourceFiles: map[build.ID]string{
			{'a'}: "../../src/main.go",
		},
		Jobs: []build.Job{
			{
				Name: "test",
				Cmds: []build.Cmd{{
					Exec: []string{"go test {{.SourceDir}}/main.go"},
				}},
				Inputs: []string{"main.go"},
			},
		},
	}

	// тут lsn - реализация интерфейса BuildListner (смотреть пакет client)
	if err := client.Build(context.Background(), graph, &lsn{}); err != nil {
		log.Fatal(err)
	}
}
```

#### 2.2 Вся система (тоже не плохо, только придется понять api соответсвующих пакетов)

```go
import (
  "gitlab.com/justnurik/distbuild/pkg/dist"
  "gitlab.com/justnurik/distbuild/pkg/client"
  "gitlab.com/justnurik/distbuild/pkg/worker"
)

func main() {
  // ----------------- Инициализация координатора -----------------
	logger, _ := zap.NewProduction()
	cache, _ := filecache.New("coordinator/filecache")
	coord := dist.NewCoordinator(logger, cache)
	defer coord.Stop()

	router := http.NewServeMux()
	router.Handle("/coordinator/", http.StripPrefix("/coordinator", coord))

	// плохо так делать, но как пример сойдет
	go func() {
		if err := http.ListenAndServe(":8080", router); err != nil {
			log.Fatal(err)
		}
	}()

  // ----------------- Запуск воркера -----------------
	fileCache, _ := filecache.New("worker/0/filecache")
	artifacts, _ := artifact.NewCache("worker/0/artifacts")

	endpointWorker := fmt.Sprintf("%s/worker/0", "localhost:6029")
	worker := worker.New(api.WorkerID(endpointWorker), endpointCoord, loggerWorker, fileCache, artifacts)

	routerWorker := http.NewServeMux()
	routerWorker.Handle(endpointWorker, http.StripPrefix(endpointWorker, worker))

	// плохо так делать, но как пример сойдет
	go func() {
		if err := http.ListenAndServe(":6029", routerWorker); err != nil {
			log.Fatal(err)
		}
	}()

  // ----------------- Запуск сборки -----------------
	loggerClient, _ := zap.NewProduction()
	client := client.NewClient(loggerClient, "", ".")

	graph := build.Graph{
		SourceFiles: map[build.ID]string{
			{'a'}: "../../src/main.go",
		},
		Jobs: []build.Job{
			{
				Name: "test",
				Cmds: []build.Cmd{{
					Exec: []string{"go test {{.SourceDir}}/main.go"},
				}},
				Inputs: []string{"main.go"},
			},
		},
	}

	// тут lsn - реализация интерфейса BuildListner (смотреть пакет client)
	if err := client.Build(context.TODO(), graph, &lsn{}); err != nil {
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
