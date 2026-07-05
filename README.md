# Taverna

Taverna - MVP+ веб-сервиса для редактирования 3D-фигурки персонажа по catalog-driven архитектуре, близкой к базовым принципам Hero Forge: каталог совместимых частей, версионированная конфигурация персонажа, 3D-preview, сохранение проекта и основа export pipeline под будущую 3D-печать.

Текущая версия остается небольшой, но уже строится вокруг backend-каталога: frontend загружает slots, parts, color zones и compatibility rules через API, пользователь собирает персонажа, сохраняет JSONB-конфигурацию в PostgreSQL и может создать fake export job как подготовку к будущему STL/GLB/3MF worker.

## Запуск

Нужно установить:

- Docker
- Docker Compose

Запуск одной командой:

```bash
docker compose up --build
```

После запуска:

- Frontend: http://localhost:5173
- Backend health check: http://localhost:8080/health

## Сервисы

- `frontend`: React + TypeScript + Three.js редактор, запускается через Vite.
- `backend`: Go HTTP-сервер с REST API, catalog seed, validation и export job API.
- `postgres`: PostgreSQL база данных для проектов и export jobs.

## Project API

Проект содержит:

- `id`
- `name`
- `config`
- `created_at`
- `updated_at`

Endpoints:

- `GET /health`: проверка backend и подключения к базе.
- `POST /api/projects`: создать проект.
- `GET /api/projects`: получить список проектов.
- `GET /api/projects/{id}`: получить один проект.
- `PUT /api/projects/{id}`: обновить проект.
- `DELETE /api/projects/{id}`: удалить проект.

Backend валидирует `config.schema_version`, наличие всех выбранных part IDs в каталоге, color zones и compatibility rules.

Пример payload:

```json
{
  "name": "Taverna hero",
  "config": {
    "schema_version": 1,
    "species": "human_01",
    "body": "balanced_body_01",
    "head": "default_head_01",
    "clothing": "basic_armor_01",
    "gear": "sword_01",
    "pose": "idle_01",
    "stage": "plain_stage_01",
    "base": "round_base_32mm",
    "scale": {
      "mm": 32
    },
    "product_target": "stl_print",
    "colors": {
      "skin": "#c99672",
      "hair": "#3b2416",
      "clothing_primary": "#444444",
      "clothing_secondary": "#8B0000",
      "gear_metal": "#d8dee9",
      "stage": "#29313d"
    }
  }
}
```

## Catalog API

Catalog endpoints:

- `GET /api/catalog`: полный каталог.
- `GET /api/catalog/slots`: slots редактора.
- `GET /api/catalog/parts`: все parts.
- `GET /api/catalog/parts?slot=gear`: parts одного slot.
- `GET /api/catalog/rules`: compatibility rules.

Catalog seed сейчас находится в backend-коде и содержит:

- slots: `species`, `body`, `head`, `clothing`, `gear`, `pose`, `stage`, `base`, `product_target`;
- parts с `preview_asset`, `render_asset`, tags, color zones и print constraints;
- color zones;
- compatibility rules, которые backend применяет при сохранении проекта.

## Export API

Export endpoints:

- `POST /api/projects/{id}/exports`: создать fake export job для проекта.
- `GET /api/exports/{id}`: получить export job.

Статусы jobs:

- `queued`
- `processing`
- `completed`
- `failed`

На текущем этапе Redis, MinIO и mesh worker намеренно не подключены. API создает job и сразу завершает его placeholder artifact metadata, чтобы зафиксировать будущий контракт.

## Что входит в MVP+

- Catalog-driven React editor.
- Backend catalog API.
- Versioned `CharacterConfig` со `schema_version: 1`.
- Compatibility validation на backend.
- Three.js preview, который реагирует на species, body, clothing, gear, pose, stage, base, scale и material zones.
- Сохранение, загрузка, открытие, обновление и удаление проектов.
- PostgreSQL таблицы `projects` и `export_jobs`.
- Fake export job API под будущий STL/GLB/3MF pipeline.
- Docker Compose для frontend, backend и PostgreSQL.

## Что не входит в этот этап

- Реальная генерация STL, 3MF или GLB.
- Blender worker.
- Redis очередь.
- MinIO файловое хранилище.
- Оплата.
- Личный кабинет, регистрация и авторизация.
- Админка заказов.
- Отправка на 3D-принтер.
- Полноценный kitbashing.

## Что можно добавить после MVP+

- Background worker для генерации экспортов.
- Redis для очереди задач.
- MinIO для сгенерированных файлов и ассетов.
- GLB placeholders вместо процедурных Three.js primitives.
- API для реального экспорта GLB, STL и 3MF.
- Пользовательскую библиотеку проектов, share links и версии персонажей.
- Админку каталога частей и заказов.
