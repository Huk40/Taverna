# Taverna

Taverna - MVP веб-сервиса для редактирования простой 3D-фигурки персонажа, сохранения JSON-конфигурации и подготовки основы для будущей генерации моделей под 3D-печать.

Текущая версия намеренно небольшая: пользователь открывает редактор, меняет параметры персонажа, видит Three.js-превью, сохраняет проект через Go REST API и загружает сохраненную конфигурацию обратно из PostgreSQL.

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
- `backend`: Go HTTP-сервер с REST API.
- `postgres`: PostgreSQL база данных для общих проектов.

## API

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

Пример payload:

```json
{
  "name": "Taverna hero",
  "config": {
    "body": "human_male_01",
    "head": "default_head_01",
    "hair": "short_hair_01",
    "armor": "basic_armor_01",
    "weapon": "sword_01",
    "pose": "idle_01",
    "base": "round_base_32mm",
    "scale_mm": 32,
    "colors": {
      "armor": "#444444",
      "hair": "#3b2416",
      "cape": "#8B0000"
    }
  }
}
```

## Что входит в MVP

- Страница редактора на React.
- Three.js сцена с камерой, светом, базой и примитивной моделью персонажа.
- Поля для `body`, `head`, `hair`, `armor`, `weapon`, `pose`, `base`, `scale_mm` и `colors`.
- Сохранение, загрузка, открытие, обновление и удаление проектов.
- Go REST API с хранением данных в PostgreSQL.
- SQL-миграция для таблицы `projects`.
- Docker Compose для frontend, backend и PostgreSQL.

## Что не входит в первый этап

- Генерация STL, 3MF или GLB.
- Blender worker.
- Redis очередь.
- MinIO файловое хранилище.
- Оплата.
- Личный кабинет, регистрация и авторизация.
- Админка заказов.
- Отправка на 3D-принтер.

## Что можно добавить после MVP

- Background worker для генерации экспортов.
- Redis для очереди задач.
- MinIO для сгенерированных файлов и ассетов.
- API для экспорта GLB, STL и 3MF.
- Админку заказов после стабилизации сохранения проектов и экспортов.
