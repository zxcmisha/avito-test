# Room Booking Service (Avito Test Task)

Go-сервис бронирования переговорок

## Запуск

```bash
docker compose up --build
```

Сервис доступен на `http://localhost:8080`.

## Миграции

Миграции выполняются автоматически при старте сервиса через
[`golang-migrate`](https://github.com/golang-migrate/migrate).

- SQL-файлы: `internal/migrations`
- В Docker используется `MIGRATIONS_PATH=file:///app/internal/migrations`

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
