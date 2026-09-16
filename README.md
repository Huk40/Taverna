# Taverna

Taverna - MVP+ веб-сервиса для редактирования 3D-фигурки персонажа по catalog-driven архитектуре, близкой к базовым принципам Hero Forge: каталог совместимых частей, версионированная конфигурация персонажа, 3D-preview, сохранение проекта и основа export pipeline под будущую 3D-печать.

Текущий интерфейс оставлен минимальным: 3D-фигурка, выбор тела и головы, высота и кнопка скачивания STL. Администратор управляет моделями на `/admin`. API сохранённых проектов и заданий экспорта сохранён для совместимости.

## Запуск

Нужно установить:

- Docker
- Docker Compose

Перед первым запуском скопируйте `.env.example` в `.env` и задайте `ADMIN_TOKEN` — случайный ключ не короче 32 символов. Например, создайте ключ командой `openssl rand -hex 32`. Файл `.env` исключён из Git. Ключ используется только backend; не помещайте его в переменные `VITE_*`.

Запуск:

```bash
docker compose up --build
```

После запуска:

- Frontend: http://localhost:5173
- Backend health check: http://localhost:8080/health

## Администратор и библиотека 3D-ассетов

1. Откройте `/admin` напрямую и введите `ADMIN_TOKEN` из локального `.env`.
2. Выберите или перетащите файл **GLB, GLTF, OBJ, STL или 3MF**. Лимит по умолчанию — **50 МиБ**; задаётся на backend через `MAX_MODEL_ASSET_BYTES`.
3. Проверьте модель в отдельном 3D-предпросмотре, задайте название, категорию и точку крепления. Категории: тело, голова, одежда/броня, оружие/предметы, окружение, подставка.
4. Нажмите **Загрузить в библиотеку**. Сервер проверяет геометрию и сохраняет файл и метаданные; модель сразу доступна через публичный API. Другие уже открытые вкладки увидят её после перезагрузки.
5. Пользователь выбирает модель прямо в **Тело** или **Голова**. Выбор сразу заменяет деталь и остаётся в текущем разделе. Основной экран содержит только 3D-сцену, выбор деталей, высоту и скачивание STL. Предпросмотр и настройка исходных моделей доступны в админке.

Управление моделями находится на отдельной странице. Можно загрузить и рассмотреть модель, изменить название и локальные точки крепления или удалить неиспользуемую модель. Сервер сохраняет защиту от удаления ассетов/якорей, на которые ссылаются сохранённые проекты.

Ключ хранится в памяти вкладки до выхода или перезагрузки. `POST`, `PUT` и `DELETE` для `/api/model-assets` защищены на сервере заголовком `Authorization: Bearer <ADMIN_TOKEN>`; скрытие кнопок не является проверкой доступа. Без настроенного ключа либо при ключе короче 32 символов управление отключено (`503`), неверный/отсутствующий ключ возвращает `401`. Просмотр библиотеки, файлов и сборка доступны без ключа. Для доступа через интернет используйте HTTPS.

- `GET /api/admin/session`: доступность входа (`configured`) и максимальный размер файла (`max_upload_bytes`).
- `POST /api/admin/session` с `Authorization: Bearer ...`: проверка ключа.

GLB предпочтителен для модели с текстурами в одном файле. GLTF должен содержать встроенные буферы/изображения; внешние зависимости загрузчик не запрашивает. OBJ поддерживается без отдельных MTL/текстур. Непустая геометрия проверяется сервером, но это не проверка пригодности к печати. Выбор тела или головы заменяет предыдущую деталь этого слота. Процедурная демо-фигурка и стартовая экипировка удалены; сцена показывает только выбранные загруженные модели и подставку. Риггинг и деформация одежды в этот этап не входят.

В `assets/starter-characters/` находятся четыре базовые GLB-модели: мужское и женское тело в одежде без головы и две сменные головы. Если тело имеет локальную точку `head_mount`, сервер крепит голову к этой точке с учётом transform тела, независимо от порядка добавления деталей. Остальные тела используют прежние точки персонажа. Описание моделей и команды генерации/загрузки — в `assets/starter-characters/README.md`. Проверка всех комбинаций тел/голов: `node scripts/check-starter-characters.mjs`.

### Проверка изменений

- Backend: `cd backend` → `go test ./...`.
- Frontend: `cd frontend` → `pnpm build`.
- STL: `node scripts/check-print-stl.mjs`. Скачивает все сочетания тел/голов в 32 и 40 мм, проверяет бинарный формат, размеры и отсутствие изменений проектов. Сохраняет два образца в `artifacts/stl/`.
- Интеграционный сценарий при запущенном локальном backend (Node.js 20.6+): `node --env-file=.env scripts/check-asset-library.mjs`. Проверяет вход, запрет анонимных изменений, загрузку/чтение/обновление, сборку, сохранение и защиту используемых ассетов; удаляет только собственные тестовые объекты.
- Для ручной проверки формы есть модель `tests/fixtures/library-sword.obj`.

## Скачать текущую фигурку в STL

Выберите тело и голову, задайте **Высота, мм**, нажмите **Скачать STL**. Высота включает подставку. Предварительное сохранение проекта не требуется.

`POST /api/exports/stl` принимает `{ "config": ... }` и возвращает бинарный `model/stl` с `Content-Disposition: attachment`. Сервер заново вычисляет крепления из текущей конфигурации, преобразует все выбранные модели, добавляет стандартную подставку (если не выбрана своя), переводит координаты в миллиметры и ориентирует модель по Z с нижней точкой на нуле. Пустая сборка и ошибка чтения любой детали не создают файл. Экспорт не записывает проекты или задания в БД.

STL содержит геометрию без цветов. Автоматическое объединение пересекающихся оболочек, ремонт незамкнутых сеток и генерация поддержек пока не выполняются; перед печатью это проверяется в слайсере. Это экспорт геометрии, а не гарантия печатной пригодности произвольного загруженного ассета.

## Сервисы

- `frontend`: React + TypeScript + Three.js, выбор моделей и скачивание STL; запускается через Vite.
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
- `POST /api/assembly/preview`: получить server-side assembly manifest для текущего unsaved `config`.
- `PUT /api/projects/{id}`: обновить проект.
- `DELETE /api/projects/{id}`: удалить проект.

Backend валидирует `config.schema_version`, наличие всех выбранных part IDs в каталоге, color zones, compatibility rules и print constraints выбранных parts. Frontend также показывает print-readiness summary и ограничивает `scale.mm` максимумом выбранных parts.

Пример payload:

```json
{
  "name": "Taverna hero",
  "config": {
    "schema_version": 1,
    "species": "",
    "body": "",
    "head": "",
    "clothing": "",
    "gear": "",
    "pose": "",
    "stage": "",
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

- slots: `body`, `head`, `clothing`, `gear`, `stage`, `base`, `product_target`; пустые категории моделей скрыты в навигации;
- настройки стандартной подставки и формата экспорта; модели персонажа поступают только из `/api/model-assets`;
- color zones;
- compatibility rules, которые backend применяет при сохранении проекта.

Старые демо-расы, пропорции, головы, одежда, оружие, позы и сцены удалены из активного каталога. В `seed/retired-parts.json` остаются только их ID для чтения старых сохранений: при декодировании такие выборы очищаются, ссылки на загруженные модели сохраняются. Новый персонаж начинает с доступных базовых тела и головы.

## Model Asset API

Model asset endpoints:

- `GET /api/model-assets`: список загруженных 3D-моделей.
- `GET /api/model-assets?slot=gear`: модели одного slot.
- `POST /api/model-assets`: multipart upload поля `file`, `label`, `slot`, `attachment_point`; требуется ключ администратора.
- `assembly_points`: optional multipart field with JSON local anchors for the uploaded model, for example `[{"id":"grip","label":"Grip","position":[0,0,0],"rotation":[0,0,0]}]`.
- `GET /api/model-assets/{id}`: получить metadata одной модели.
- `PUT /api/model-assets/{id}`: обновить `label` и/или `assembly_points` без повторной загрузки файла. Если новая версия удаляет или переименовывает anchor id, который уже указан в сохраненных `projects.config.assembly.placements`, backend возвращает `409 Conflict` со списком зависимых placements.
- `GET /api/model-assets/{id}/file`: получить файл модели для Three.js preview.
- `DELETE /api/model-assets/{id}`: удалить metadata и файл. Если asset уже используется в сохраненных `projects.config.assembly.placements`, backend возвращает `409 Conflict`, чтобы не сломать server assembly manifest/export у существующих проектов.
- `GET /api/model-assets/assembly-points`: список поддержанных target points для сборки.

Поддержанные форматы upload: `.glb`, `.gltf`, `.obj`, `.stl`, `.3mf`. Для `.gltf` MVP поддерживает embedded buffers через data URI; standalone `.gltf` с внешним `.bin` отклоняется, потому что сервер получает только один файл upload-а. Файлы хранятся локально в backend storage volume, metadata хранится в PostgreSQL таблице `model_assets`. Проект сохраняет ссылки на загруженные модели в `config.assembly.placements`, а backend проверяет, что referenced asset IDs существуют.

`config.assembly.placements[].asset_point_id` selects the local anchor of the uploaded model. The server aligns that asset anchor to the character `attachment_point` and includes the resolved `asset_point` plus final `world_transform` in the assembly manifest.

Admin uploads populate the shared library. Adding a library model to a character creates a `config.assembly.placements` entry at its configured `attachment_point` with a bounds-derived initial scale.

When an uploaded OBJ/STL/GLB/GLTF/3MF asset has parsed bounds, the asset anchor editor offers position presets such as center, bottom, top, left, right, front, and back. These presets are stored as normal `assembly_points` and make it faster to align the model to character sockets.

When uploaded OBJ/STL/GLB/GLTF/3MF assets have parsed geometry metadata, the assembly manifest also includes per-placement `world_bounds` and combined manifest `bounds`, both computed from the resolved `world_transform`.

Uploaded model asset responses include `metadata` with server-inspected geometry for OBJ, embedded-bin GLB, embedded-buffer GLTF, ASCII/binary STL, and simple 3MF mesh files: parser status, source type, bounds (`min`, `max`, `size`, `center`), vertex count, face count, triangle count, and warnings.

## Assembly Manifest API

`GET /api/projects/{id}/assembly` резолвит сохраненный `config.assembly.placements` в server-side manifest:

- выбранные catalog slots проекта;
- `scale_mm` и `character_scale`;
- uploaded asset metadata и file URL;
- selected `asset_point_id` and resolved local `asset_point`;
- socket transform точки сборки;
- пользовательский transform placement-а;
- итоговый `world_transform`, который будущий mesh worker сможет использовать для сборки GLB/GLTF/STL/3MF.

`POST /api/assembly/preview` принимает `{ "config": CharacterConfig }` и возвращает такой же manifest с `project_id: "preview"`, чтобы frontend мог проверить текущую сборку по точкам до сохранения проекта.

Frontend автоматически обновляет этот preview manifest при изменении `config.assembly.placements` и использует server-side `world_transform` в Three.js viewport, поэтому положение загруженных моделей в редакторе соответствует тому же transform contract, который использует export pipeline.

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

Для первого физического artifact pipeline backend поддерживает `format: "obj"` и `format: "stl"`. OBJ export берет uploaded OBJ assets из assembly manifest, применяет рассчитанные `world_transform`, склеивает вершины/faces в один `.obj` и сохраняет файл в server storage. STL export создает ASCII `.stl`: uploaded ASCII/binary STL перечитываются как facets, простые OBJ placements триангулируются, а embedded-bin GLB, embedded-buffer GLTF и simple 3MF mesh placements конвертируются в STL facets с пересчитанными normals. Полноценная генерация GLB/GLTF/3MF output остается задачей будущего worker-а.

## Share links

После сохранения проекта frontend формирует ссылку вида `http://localhost:5173/?project={id}`. При открытии такой ссылки приложение загружает каталог, получает проект через `GET /api/projects/{id}` и восстанавливает конфигурацию персонажа вместе с историей export jobs.

## Что входит в MVP+

- Catalog-driven React editor shell с category nav, part cards и отдельными panels.
- Backend catalog API.
- Versioned `CharacterConfig` со `schema_version: 1`.
- Admin-only model asset upload API с локальным server storage и общей библиотекой для пользователей.
- Upload UI for multiple local asset anchors, persisted as `assembly_points`.
- Server-inspected geometry metadata for uploaded OBJ, embedded-bin GLB, embedded-buffer GLTF, ASCII/binary STL, and simple 3MF mesh assets.
- Assembly placements по точкам сборки (`right_hand`, `left_hand`, `head_mount`, `back_mount`, `base_mount`, `stage_anchor`).
- Server-side assembly manifest API для будущего mesh worker.
- Server-side assembled bounds in the assembly manifest for parsed uploaded assets.
- MVP OBJ/STL artifact builder: собирает uploaded OBJ placements в downloadable `.obj`, а uploaded ASCII/binary STL, embedded-bin GLB, embedded-buffer GLTF, simple 3MF mesh и triangulated OBJ placements - в downloadable ASCII `.stl`.
- Compatibility validation на backend.
- Print-readiness summary на frontend с динамическим максимумом `scale.mm`.
- Three.js preview, который реагирует на species, body, clothing, gear, pose, stage, base, scale, material zones и uploaded assembly assets.
- Сохранение, загрузка, открытие, обновление и удаление проектов.
- Share link для открытия сохраненного проекта по URL.
- PostgreSQL таблицы `projects`, `model_assets` и `export_jobs`.
- Fake export job API под будущий STL/GLB/GLTF/3MF pipeline.
- Docker Compose для frontend, backend и PostgreSQL.

## Что не входит в этот этап

- Полноценная генерация GLB/GLTF/3MF и сложная STL-конвертация через mesh worker.
- Blender worker или отдельный mesh worker service.
- Redis очередь.
- MinIO файловое хранилище.
- Auth-scoped personal asset library.
- Оплата.
- Личный кабинет, регистрация и авторизация пользователей (доступ администратора к ассетам реализован через серверный ключ).
- Админка заказов.
- Отправка на 3D-принтер.
- Полноценный kitbashing.

## Что можно добавить после MVP+

- Background worker для генерации экспортов.
- Redis для очереди задач.
- MinIO для сгенерированных файлов и ассетов.
- GLB placeholders вместо процедурных Three.js primitives.
- API для реального экспорта GLB, GLTF, STL и 3MF.
- Пользовательскую библиотеку проектов, share links и версии персонажей.
- Админку каталога частей и заказов.
