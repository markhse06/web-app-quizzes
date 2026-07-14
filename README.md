# Quiz MVP

MVP веб-приложения для проведения квизов в реальном времени — аналог Kahoot. Организатор создаёт квиз и управляет игрой, участники входят в комнату по PIN-коду и отвечают с телефона.

## Стек

- Backend: Go, Gin, GORM, PostgreSQL, Gorilla WebSocket.
- Frontend: React, Vite, React Router.
- Инфраструктура: Docker Compose, Nginx (production-раздача фронтенда).

## Архитектура

```text
Браузер
  └─ frontend (Nginx, :5173)
       ├─ React SPA
       └─ proxy: /api, /ws, /uploads
            └─ backend (Gin, :8080)
                 ├─ REST API + JWT для организатора
                 ├─ WebSocket Hub с комнатами в памяти
                 ├─ uploads/ с изображениями вопросов
                 └─ PostgreSQL (:5432)
```

### Роли

| Роль | Вход | Возможности |
| --- | --- | --- |
| Host / организатор | Email + пароль, Access/Refresh JWT | Создаёт квизы, запускает сессии, управляет вопросами. |
| Player / участник | PIN комнаты + имя | Подключается к игре и отвечает на вопросы. |

### Основные сущности

- `User` — организатор.
- `Quiz` → `Question` → `Answer` — шаблон квиза.
- `GameSession` — запущенная комната с уникальным шестизначным PIN.
- `Participant` и `PlayerAnswer` — участник и выбранные им варианты.
- `Hub`/`Room` — состояние активной игры в памяти backend: подключения, вопрос, таймер и broadcast-канал.

> Активные WebSocket-комнаты хранятся в памяти backend. После перезапуска сервера текущие соединения игроков нужно открыть заново.

## Быстрый запуск в Docker

Из корня проекта:

```bash
docker compose up --build
```

После запуска:

- приложение: `http://localhost:5173`;
- backend health-check: `http://localhost:8080/health`;
- PostgreSQL: `localhost:5432`.

Compose создаёт два именованных volume:

- `pgdata` — данные PostgreSQL;
- `uploads` — изображения вопросов.

Для остановки используйте:

```bash
docker compose down
```

## Локальный запуск без Docker

1. Поднимите PostgreSQL и задайте переменные из `backend/.env.example`.
2. Запустите backend:

   ```bash
   cd backend
   go run .
   ```

3. В отдельном терминале запустите frontend:

   ```bash
   cd frontend
   npm install
   npm run dev
   ```

Vite проксирует `/api`, `/ws` и `/uploads` на `http://localhost:8080`. При необходимости задайте `VITE_BACKEND_URL`.

## HTTP API

Все JSON-запросы используют `Content-Type: application/json`. Защищённые endpoints требуют заголовок:

```http
Authorization: Bearer <access_token>
```

### Аутентификация организатора

| Метод и путь | Авторизация | Назначение |
| --- | --- | --- |
| `POST /api/user/register` | нет | Регистрация организатора. |
| `POST /api/user/login` | нет | Вход и получение пары JWT-токенов. |
| `POST /api/auth/refresh` | нет | Ротация refresh-токена. |

Пример регистрации:

```json
{
  "email": "host@example.com",
  "password": "strong-password"
}
```

Ответ `POST /api/user/login`:

```json
{
  "access_token": "<jwt>",
  "refresh_token": "<token>",
  "user": { "id": "<uuid>", "email": "host@example.com" }
}
```

Для refresh отправьте:

```json
{ "refresh_token": "<token>" }
```

### Квизы

| Метод и путь | Назначение |
| --- | --- |
| `POST /api/quizzes` | Создать квиз с вопросами и вариантами. |
| `GET /api/quizzes` | Список квизов текущего организатора. |
| `GET /api/quizzes/:id` | Квиз с вопросами и вариантами. |
| `DELETE /api/quizzes/:id` | Удалить собственный квиз. |

Пример создания:

```json
{
  "title": "Космический квиз",
  "question_duration": 20,
  "questions": [
    {
      "content": "Какая планета ближе всего к Солнцу?",
      "type": "single",
      "image_path": "/uploads/example.jpg",
      "answers": [
        { "content": "Меркурий", "is_correct": true },
        { "content": "Венера", "is_correct": false }
      ]
    }
  ]
}
```

`type` принимает `single` или `multiple`. Для множественного выбора ответ верен, только если участник выбрал полный набор правильных вариантов.

### Изображения

| Метод и путь | Назначение |
| --- | --- |
| `POST /api/upload` | Загрузить изображение вопроса. |

Endpoint принимает `multipart/form-data` с полем `file`. Поддерживаются `.jpg`, `.jpeg`, `.png`, `.gif`, `.webp`.

Ответ:

```json
{ "path": "/uploads/6c72882e-2df0-4f6f-a63a-a1a0e1ba0b23.png" }
```

Файл доступен по этому же пути через frontend proxy.

### Игровые сессии

| Метод и путь | Назначение |
| --- | --- |
| `POST /api/sessions` | Создать комнату для собственного квиза. |

Запрос:

```json
{ "quiz_id": "<uuid>" }
```

Ответ:

```json
{
  "session_id": "<uuid>",
  "pin_code": "123456",
  "status": "waiting"
}
```

## WebSocket API

Подключение: `GET /ws/join?pin=<PIN>&name=<name>`.

### Подключение игрока

Игрок подключается без токена:

```text
ws://localhost:8080/ws/join?pin=123456&name=Мария
```

После подключения сервер отправляет личное событие:

```json
{
  "type": "joined",
  "participant": { "id": "<uuid>", "name": "Мария", "score": 0 }
}
```

### Подключение хоста

Хост подключается с `name=host`, но обязан передать Access JWT в WebSocket subprotocol:

```js
new WebSocket(
  'ws://localhost:8080/ws/join?pin=123456&name=host',
  ['access_token', accessToken],
)
```

Backend проверяет токен и что пользователь является создателем квиза. Нельзя получить host-права только именем `host`.

### Сообщения клиента

| `type` | Кто отправляет | Дополнительные поля | Назначение |
| --- | --- | --- | --- |
| `start_game` | host | — | Запустить первый вопрос. |
| `submit_answer` | player | `answer_ids: ["<uuid>"]` | Сохранить выбранные варианты. |
| `question_time_over` | host | — | Досрочно завершить вопрос. |
| `next_question` | host | — | Перейти к следующему вопросу или завершить игру. |

### Сообщения сервера

| `type` | Основные поля | Значение |
| --- | --- | --- |
| `lobby_update` | `players` | Актуальный список подключённых игроков. |
| `question` | `question`, `duration_seconds` | Вопрос без поля `is_correct`. |
| `question_result` | `correct_answer_ids`, `answer_counts`, `leaderboard` | Результаты вопроса и промежуточный рейтинг. |
| `game_finished` | — | Игра завершена. |

`answer_counts` — объект, где ключом служит ID варианта, а значением — число голосов.

## Как пользоваться приложением

### Организатор

1. Откройте `http://localhost:5173/register` и создайте аккаунт.
2. Войдите на странице `/login`.
3. В кабинете нажмите «Создать квиз», добавьте вопросы, варианты и при необходимости изображения, затем сохраните.
4. В кабинете нажмите «Запустить игру» рядом с квизом.
5. Покажите участникам PIN на экране лобби.
6. Нажмите «Начать игру», следите за таймером и при необходимости завершайте вопрос досрочно.
7. После результатов нажимайте «Следующий вопрос». После последнего вопроса игра завершится.

### Участник

1. Откройте `http://localhost:5173` на телефоне или компьютере.
2. Введите PIN комнаты и имя.
3. Дождитесь старта игры.
4. Выберите вариант. Для вопроса с множественным выбором отметьте несколько вариантов и нажмите «Подтвердить ответ».
5. После каждого вопроса смотрите личный результат; в финале — место и общий счёт.

## Проверки

```bash
# backend
cd backend
go test ./...

# frontend
cd frontend
npm run lint
npm run build
```

Интеграционные тесты backend используют временный PostgreSQL-контейнер через Testcontainers, поэтому для их запуска нужен Docker.
