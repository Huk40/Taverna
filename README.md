# Развёртывание Taverna

## Требования

Docker Engine / Docker Desktop и Docker Compose 2.24.4 или новее. Для запуска готового приложения Go и Node.js на компьютере не нужны.

## Локальный запуск

Из корня репозитория:

```sh
cp .env.example .env
docker compose up -d --build --wait
```

Если файл .env уже существует, сохраните его настройки. Приложение: http://localhost:5173, проверка доступности: http://localhost:5173/health. Локальные письма подтверждения и восстановления пароля: http://localhost:8025 (Mailpit).

Frontend собирается и раздаётся Nginx; запросы /api идут на Go-сервер через тот же адрес. PostgreSQL и прямой доступ к backend опубликованы только на localhost:5432 и localhost:8080.

Миграции базы применяются автоматически при запуске backend. При конфликте сети задайте в .env свободный NETWORK_PREFIX, например 172.30.25.

## Администратор и стартовые модели

1. Зарегистрируйте аккаунт в приложении.
2. Назначьте ему роль администратора:

```sh
docker compose exec backend /app/taverna-admin -email owner@example.com
```

3. Для установки четырёх готовых моделей добавьте в .env TEST_ADMIN_EMAIL и TEST_ADMIN_PASSWORD этого аккаунта и выполните:

```sh
docker compose --profile tools run --rm seed
```

Повторный запуск использует уже загруженные идентичные модели. После установки удалите TEST_ADMIN_PASSWORD из .env. Собственные GLB, GLTF, OBJ, STL и 3MF загружаются через /admin. Лимит файла — 50 МиБ; сервер также ограничивает сложность геометрии.

У каждой сохранённой сборки должен быть владелец. При обновлении миграция 007 удаляет старые сборки без владельца и связанные записи экспортов. Сборки пользователей и файлы библиотеки моделей сохраняются.

## Сервер с HTTPS

Направьте DNS-запись домена на сервер и откройте входящие TCP-порты 80 и 443. Для HTTP/3 можно открыть UDP 443. В .env задайте:

```dotenv
APP_DOMAIN=taverna.example.com
POSTGRES_PASSWORD=replace_with_random_hex_password
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USERNAME=your_smtp_user
SMTP_PASSWORD=your_smtp_password
SMTP_FROM=noreply@example.com
```

POSTGRES_PASSWORD задайте до первого запуска базы: используйте случайный пароль из латинских букв и цифр, чтобы он подходил для URL подключения. Изменение этой переменной не меняет пароль существующей базы.

Запуск:

```sh
docker compose -f docker-compose.yml -f docker-compose.production.yml up -d --build --wait
```

Caddy получает и продлевает сертификат. Production-конфигурация включает Secure-cookie, точный разрешённый origin и обязательный STARTTLS для SMTP. Откройте https://taverna.example.com. Для назначения администратора и загрузки моделей используйте команды выше с теми же двумя параметрами -f.

Не публикуйте .env в Git. Mailpit остаётся доступен только с сервера по localhost:8025; для настоящих писем используются указанные SMTP-настройки.

## Обновление и диагностика

После обновления исходников повторите команду запуска с --build --wait. При HTTPS-развёртывании всегда указывайте оба Compose-файла.

```sh
docker compose ps
docker compose logs --tail=100 backend frontend
docker compose down
```

Обычная остановка сохраняет данные. Не используйте down -v для обновления: он удаляет volumes.

## Резервное копирование

Сохраняйте согласованную копию базы и файлов моделей/экспортов. На время копирования остановите запись:

```sh
docker compose stop backend
docker compose exec postgres pg_dump -U taverna -d taverna -Fc -f /tmp/taverna.dump
docker compose cp postgres:/tmp/taverna.dump ./taverna.dump
docker compose cp backend:/app/storage ./storage-backup
docker compose start backend
```

Храните резервные копии вне сервера. В HTTPS-развёртывании сертификаты находятся в volumes caddy_data и caddy_config.
