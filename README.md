# Taverna

Taverna - MVP+ веб-сервиса для редактирования 3D-фигурки персонажа по catalog-driven архитектуре, близкой к базовым принципам Hero Forge: каталог совместимых частей, версионированная конфигурация персонажа, 3D-preview, сохранение проекта и основа export pipeline под будущую 3D-печать.

Текущая версия остается небольшой, но уже строится вокруг backend-каталога: frontend загружает slots, parts, color zones, compatibility rules и пользовательские 3D assets через API, пользователь собирает персонажа в Hero Forge-style editor shell с категориями, карточками parts и assembly placements, сохраняет JSONB-конфигурацию в PostgreSQL, открывает проект по share link и может создать fake export job как подготовку к будущему STL/GLB/3MF worker.

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

- `frontend`: React + TypeScript + Three.js editor shell с категориями, part cards, asset upload, assembly/color/project/export panels; запускается через Vite.
- `backend`: Go HTTP-сервер с REST API, catalog seed, model asset upload/storage, validation и export job API.
- `postgres`: PostgreSQL база данных для проектов, model assets и export jobs.

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
- `GET /api/projects/{id}/assembly`: получить server-side assembly manifest для сохраненной конфигурации.
- `PUT /api/projects/{id}`: обновить проект.
- `DELETE /api/projects/{id}`: удалить проект.

Backend валидирует `config.schema_version`, наличие всех выбранных part IDs в каталоге, color zones, compatibility rules и print constraints выбранных parts. Frontend также показывает print-readiness summary и ограничивает `scale.mm` максимумом выбранных parts.

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
    },
    "assembly": {
      "placements": [
        {
          "id": "placement-1",
          "slot": "gear",
          "attachment_point": "right_hand",
          "asset_id": "uploaded-model-uuid",
          "transform": {
            "position": [0, 0, 0],
            "rotation": [0, 0, 0],
            "scale": 1
          }
        }
      ]
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

Catalog seed сейчас находится в `backend/internal/catalog/seed/catalog.json`, встраивается в Go-бинарник через `embed` и содержит:

- slots: `species`, `body`, `head`, `clothing`, `gear`, `pose`, `stage`, `base`, `product_target`;
- parts с `preview_asset`, `render_asset`, tags, color zones и print constraints;
- color zones;
- compatibility rules, которые backend применяет при сохранении проекта.

## Model Asset API

Model asset endpoints:

- `GET /api/model-assets`: список загруженных 3D-моделей.
- `GET /api/model-assets?slot=gear`: модели одного slot.
- `POST /api/model-assets`: multipart upload поля `file`, `label`, `slot`, `attachment_point`.
- `assembly_points`: optional multipart field with JSON local anchors for the uploaded model, for example `[{"id":"grip","label":"Grip","position":[0,0,0],"rotation":[0,0,0]}]`.
- `GET /api/model-assets/{id}`: получить metadata одной модели.
- `GET /api/model-assets/{id}/file`: получить файл модели для Three.js preview.
- `DELETE /api/model-assets/{id}`: удалить metadata и файл.
- `GET /api/model-assets/assembly-points`: список поддержанных target points для сборки.

Поддержанные форматы upload: `.glb`, `.gltf`, `.obj`, `.stl`, `.3mf`. Файлы хранятся локально в backend storage volume, metadata хранится в PostgreSQL таблице `model_assets`. Проект сохраняет ссылки на загруженные модели в `config.assembly.placements`, а backend проверяет, что referenced asset IDs существуют.

`config.assembly.placements[].asset_point_id` selects the local anchor of the uploaded model. The server aligns that asset anchor to the character `attachment_point` and includes the resolved `asset_point` plus final `world_transform` in the assembly manifest.

## Assembly Manifest API

`GET /api/projects/{id}/assembly` резолвит сохраненный `config.assembly.placements` в server-side manifest:

- выбранные catalog slots проекта;
- `scale_mm` и `character_scale`;
- uploaded asset metadata и file URL;
- selected `asset_point_id` and resolved local `asset_point`;
- socket transform точки сборки;
- пользовательский transform placement-а;
- итоговый `world_transform`, который будущий mesh worker сможет использовать для сборки GLB/STL/3MF.

Этот endpoint не генерирует новый mesh-файл сам по себе, но фиксирует серверный контракт сборки: какой пользовательский файл, в какой attachment point и с каким transform должен быть поставлен.

## Export API

Export endpoints:

- `POST /api/projects/{id}/exports`: создать fake export job для проекта.
- `GET /api/projects/{id}/exports`: получить историю export jobs проекта.
- `GET /api/exports/{id}`: получить export job.
- `GET /api/exports/{id}/artifact`: скачать созданный artifact-файл, если export format его поддерживает.

Статусы jobs:

- `queued`
- `processing`
- `completed`
- `failed`

На текущем этапе Redis, MinIO и отдельный mesh worker намеренно не подключены. API создает job, сразу завершает его metadata, включает краткий assembly summary и полный `assembly_manifest` по uploaded assets, затем сохраняет историю export jobs проекта, чтобы зафиксировать будущий контракт.

Для первого физического artifact pipeline backend поддерживает `format: "obj"` и `format: "stl"`. OBJ export берет uploaded OBJ assets из assembly manifest, применяет рассчитанные `world_transform`, склеивает вершины/faces в один `.obj` и сохраняет файл в server storage. STL export создает ASCII `.stl`: uploaded STL перечитываются как facets, а простые OBJ placements триангулируются в STL facets с пересчитанными normals. Для `.glb` и `.3mf` пока возвращается manifest-only artifact с предупреждением; полноценная конвертация этих форматов остается задачей будущего worker-а.

## Share links

После сохранения проекта frontend формирует ссылку вида `http://localhost:5173/?project={id}`. При открытии такой ссылки приложение загружает каталог, получает проект через `GET /api/projects/{id}` и восстанавливает конфигурацию персонажа вместе с историей export jobs.

## Что входит в MVP+

- Catalog-driven React editor shell с category nav, part cards и отдельными panels.
- Backend catalog API.
- Versioned `CharacterConfig` со `schema_version: 1`.
- User model asset upload API с локальным server storage.
- Upload UI for multiple local asset anchors, persisted as `assembly_points`.
- Assembly placements по точкам сборки (`right_hand`, `left_hand`, `head_mount`, `back_mount`, `base_mount`, `stage_anchor`).
- Server-side assembly manifest API для будущего mesh worker.
- MVP OBJ/STL artifact builder: собирает uploaded OBJ placements в downloadable `.obj`, а uploaded STL и triangulated OBJ placements - в downloadable ASCII `.stl`.
- Compatibility validation на backend.
- Print-readiness summary на frontend с динамическим максимумом `scale.mm`.
- Three.js preview, который реагирует на species, body, clothing, gear, pose, stage, base, scale, material zones и uploaded assembly assets.
- Сохранение, загрузка, открытие, обновление и удаление проектов.
- Share link для открытия сохраненного проекта по URL.
- PostgreSQL таблицы `projects`, `model_assets` и `export_jobs`.
- Fake export job API под будущий STL/GLB/3MF pipeline.
- Docker Compose для frontend, backend и PostgreSQL.

## Что не входит в этот этап

- Полноценная генерация GLB/3MF и сложная STL-конвертация через mesh worker.
- Blender worker или отдельный mesh worker service.
- Redis очередь.
- MinIO файловое хранилище.
- Auth-scoped personal asset library.
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
