# iHealth Export

Личный экспорт данных Apple Health с iPhone на NAS и доступ к ним через MCP.

## Состав

- `ios/IHealthExporter.xcodeproj` — приложение iOS 26 на SwiftUI/HealthKit;
- `cmd/ihealth-server` — один Go-процесс: загрузка, SQLite и MCP;
- `docker-compose.yml` — запуск на NAS `linux/amd64`.

Приложение вручную читает доступные типы HealthKit. Первый запуск передаёт историю целиком, последующие используют `HKAnchoredObjectQuery` и отправляют только новые, изменённые и удалённые записи. Повторная полная выгрузка безопасна: записи обновляются по UUID HealthKit.

## Запуск сервера на NAS

```bash
cp .env.example .env
openssl rand -hex 32
```

Результат второй команды записать в `.env` как `IHEALTH_TOKEN`, затем:

Готовый образ для `linux/amd64` публикуется из `main`:

```bash
docker-compose pull
docker-compose up -d
curl http://NAS_IP:8080/healthz
```

Для локальной сборки вместо скачивания:

```bash
docker-compose up -d --build
```

На arm64 Mac, если legacy Docker builder падает при исполнении amd64 Go через QEMU:

```bash
make docker-build-local
```

SQLite хранится в Docker volume `ihealth-data`. Порт меняется через `IHEALTH_PORT`.

Образ: `ghcr.io/ilyalaletin/ihealth-export:latest`. Первый пакет GHCR по умолчанию приватный; для анонимного скачивания с NAS нужно один раз открыть настройки пакета на GitHub и выбрать `Change visibility → Public`. Либо выполнить на NAS `docker login ghcr.io`.

## Установка на iPhone без платной учётной записи

1. Открыть `ios/IHealthExporter.xcodeproj` в Xcode 26.
2. В `Signing & Capabilities` выбрать свой `Personal Team`.
3. Подключить iPhone, выбрать его как устройство запуска и нажать Run.
4. В приложении указать `http://NAS_IP:8080`, тот же токен и нажать «Синхронизировать».
5. Разрешить чтение данных в системном окне HealthKit.

Профиль бесплатной команды истекает через 7 дней. После этого приложение нужно снова установить из Xcode. Сервер и уже загруженные данные от этого не зависят.

## MCP

Endpoint: `http://NAS_IP:8080/mcp`  
Transport: Streamable HTTP  
Заголовок: `Authorization: Bearer <IHEALTH_TOKEN>`

Инструменты:

- `health_list_types` — типы, единицы, количество и диапазоны дат;
- `health_query` — сырые записи по типу, виду, датам и активности;
- `health_list_workouts` — тренировки по датам и виду активности;
- `health_summary` — `sum`, `avg`, `min`, `max`, `count` по часу, дню или месяцу;
- `health_profile` — дата рождения и доступные характеристики;
- `health_sync_status` — объём данных и последняя синхронизация.

Проверка MCP:

```bash
curl -X POST http://NAS_IP:8080/mcp \
  -H 'Authorization: Bearer TOKEN' \
  -H 'Accept: application/json, text/event-stream' \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"curl","version":"1"}}}'
```

Фильтры дат принимают `YYYY-MM-DD` или RFC 3339. Для тренировок можно передать `activity_name`, например `running`, `walking`, `cycling`, `swimming`, либо числовой `activity_type` HealthKit.

## Границы первой версии

- Читаются все публичные quantity/category-типы из iOS 26 SDK, тренировки, ЭКГ, аудиограммы, события приёма лекарств, эмоциональное состояние, оценки GAD-7/PHQ-9, корреляции и series-записи.
- Для основных числовых показателей сохраняются значения в канонической единице. Для редких сложных типов сохраняются общие поля, метаданные и текстовое представление объекта.
- Точки маршрутов тренировок, отсчёты ЭКГ/heartbeat series и Clinical Health Records пока не разворачиваются в отдельные записи. Clinical Health Records требуют отдельного специального entitlement Apple.
- Синхронизация только ручная; фоновой доставки нет.

## Безопасность

HTTP по локальному IP поддержан намеренно. При таком режиме токен и медицинские данные идут по локальной сети без шифрования. Если сеть не полностью доверенная, включить существующий reverse proxy с HTTPS и указать его URL в приложении. Сервер требует Bearer-токен и проверяет `Origin` для MCP-запросов.

Подробнее: [docs/architecture.md](docs/architecture.md).
