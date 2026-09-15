# sap_segmentation

## Техническое задание

Требуется разработать Go-модуль, который импортирует данные из сторонней ERP-системы в таблицу PostgreSQL.

Таблица должна содержать:

- автоинкрементируемое уникальное поле;
- `address_sap_id` — `varchar(255)`;
- `adr_segment` — `varchar(16)`;
- `segment_id` — `bigint`.

Для `address_sap_id` необходимо предусмотреть ограничение уникальности. При повторном импорте существующие записи должны обновляться через `ON CONFLICT (address_sap_id)`.

Конфигурация должна загружаться при помощи `github.com/kelseyhightower/envconfig`.

| Переменная | Значение по умолчанию | Назначение |
| --- | --- | --- |
| `DB_HOST` | `127.0.0.1` | IP-адрес сервера БД |
| `DB_PORT` | `5432` | TCP-порт сервера БД |
| `DB_NAME` | `mesh_group` | Название БД |
| `DB_USER` | `postgres` | Пользователь БД |
| `DB_PASSWORD` | `postgres` | Пароль пользователя БД |
| `CONN_URI` | `http://bsm.api.iql.ru/ords/bsm/segmentation/get_segmentation` | Адрес внешнего API |
| `CONN_AUTH_LOGIN_PWD` | `4Dfddf5:jKlljHGH` | Логин и пароль для Basic Authentication |
| `CONN_USER_AGENT` | `spacecount-test` | User-Agent для подключения к SAP |
| `CONN_TIMEOUT` | `5` | Таймаут подключения к API |
| `CONN_INTERVAL` | `1500` | Задержка между запросами, мс |
| `IMPORT_BATCH_SIZE` | `50` | Размер получаемой пачки |
| `LOG_CLEANUP_MAX_AGE` | `7` | Срок хранения логов, дней |

При импорте необходимо логировать запрашиваемый endpoint одновременно в консоль и файл `/log/segmentation_import.log`.

При запуске необходимо удалить из `/log/` файлы старше `LOG_CLEANUP_MAX_AGE` дней. Ошибки получения JSON и записи данных в БД должны попадать в лог.

Подключение к БД необходимо выполнять через `github.com/jmoiron/sqlx`, используя `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER` и `DB_PASSWORD`.

При обращении к API необходимо передавать:

- `User-Agent`, равный `CONN_USER_AGENT`;
- заголовок `Authorization: Basic XXX`, где `XXX` — Base64-кодировка `CONN_AUTH_LOGIN_PWD`.

HTTP-клиент должен использовать таймаут `CONN_TIMEOUT`.

Endpoint из `CONN_URI` поддерживает параметры:

- `p_limit` — значение `IMPORT_BATCH_SIZE`;
- `p_offset` — смещение.

Пример запроса:

```text
http://bsm.api.iql.ru/ords/bsm/segmentation/get_segmentation?p_limit=50&p_offset=1
```

Получение данных должно выполняться в цикле пачками размером `IMPORT_BATCH_SIZE`:

```text
p_limit=50&p_offset=1
p_limit=50&p_offset=50
p_limit=50&p_offset=100
p_limit=50&p_offset=150
```

Импорт завершается, когда тело ответа становится пустым.

Проект должен содержать:

- модель `model/Segmentation.go`;
- SQL-миграцию `setup/install.sql` для создания таблицы `segmentation`;
- основной файл `cmd/sap_segmentationd/main.go`.

Название Go-модуля: `sap_segmentation`. СУБД: PostgreSQL.

## Запуск

Для локального запуска требуется Docker Desktop с запущенным Linux engine и Docker Compose.

Откройте PowerShell в корне проекта и выполните одну команду:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\local-check.ps1
```

Команда автоматически:

1. соберёт Docker-образы;
2. запустит PostgreSQL и тестовый ERP API;
3. применит `setup/install.sql`;
4. выполнит импорт и повторный импорт;
5. запустит unit-, race-, integration- и end-to-end-тесты;
6. удалит тестовые контейнеры и тома после завершения.

Успешный результат содержит сообщения:

```text
E2E PASSED
PASS
```

Для Linux и macOS используется команда:

```sh
sh scripts/local-check.sh
```
