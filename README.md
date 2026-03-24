# Room Booking Service (Avito Test Task)

Go-сервис бронирования переговорок

## Запуск

```bash
make up
```

Сервис доступен на `http://localhost:8080`.

## Миграции

Миграции выполняются автоматически при старте сервиса через
[`golang-migrate`](https://github.com/golang-migrate/migrate).

## Тесты

```bash
go test ./... -cover
```

## Нагрузочное тестирование

- Запуск:

```bash
make loadtest
```

- Сценарий и краткие результаты:
  - `loadtest/RESULTS.md`
  - raw benchmark output: `loadtest/benchmark.txt`

## Сваггер 

```bash
make swagger
make up
```

Открыть UI:
http://localhost:8080/swagger/index.html