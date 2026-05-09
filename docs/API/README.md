# 📘 1. Общая архитектура
## 🧩 Тип архитектуры

Modular Monolith (с элементами event-driven)

Основные компоненты:
HTTP API (Gin)
WebSocket сервер
Kafka (event distribution)
PostgreSQL (primary storage)
Redis (cache + session)
File storage (cold storage сообщений)

## 🏗️ Архитектурные слои

```
Adapters (HTTP / WS)
↓
Services (Business logic)
↓
Storage (DB / Redis / Kafka / File)
```

## 📦 Модули системы
Модуль	  Назначение
auth	    2FA, токены
user	    пользователи, сессии
chat	    комнаты, участники
message	  сообщения, WS, Kafka
pkg	      инфраструктура

# 🔐 2. AUTH MODULE
## 📡 API
### POST /api/v1/auth/verify

Проверка 2FA кода

Request:
```
{
  "code": "123456",
  "temp_token": "jwt"
}
```

Response:
```
{
  "access_token": "...",
  "refresh_token": "..."
}
```

### POST /api/v1/auth/enable

Включение 2FA

## ⚙️ Service: TwoFA
### Методы:
`EnableTwoFA(userID int64)`
- включает 2FA
`UsersSendEmailCode(tempToken string)`
- генерирует код
- rate limit (3/15 мин)
- кеширует temp token в Redis
`VerifyCode(code domain.Code)`
- проверка кода
- ограничения:
  - max 5 попыток / 10 мин
  - max 3 попытки на код
- выдает access + refresh токены

## ⚠️ Проблемы / незавершенные части
- ❌ `DisableTwoFA` — не реализован
- ❌ Email отправка — заглушка (`fmt.Printf`)
- ⚠️ нет защиты brute-force через IP
- ⚠️ temp_token не инвалидируется

# 💬 3. CHAT MODULE
## 📡 API
### POST /chat/rooms

Создание комнаты

### GET /chat/rooms/:room_id

Получить комнату

### GET /chat/rooms/:room_id/messages

Получить сообщения

### POST /chat/messages

Отправить сообщение

## 🧠 RoomService
### Основные функции:
`CreateRoom`
- валидирует
- добавляет owner в members
- сохраняет в PG + cache
`GetRoom`
- cache → fallback DB
`CanSendMessage`
Логика прав:
- personal → только owner
- private/group → все
- video → только если call active

## ⚠️ Проблемы
- ❌ `UpdateRoom/DeleteRoom/ListUserRooms` не подключены в API
- ❌ нет ACL / ролей
- ⚠️ `isCallActive` — заглушка

# 👤 4. USER CLIENT MODULE
## 🧠 UserClientService
Методы:
### GetUser
- cache → DB fallback

### UpdateLastActivity
- обновляет timestamp
### ValidateUserPermissions
- проверка прав (primitive RBAC)
### GetUserStatus
- online / away / offline

## ⚠️ Проблемы
- ❌ permissions не загружаются из DB
- ❌ нет real RBAC
- ⚠️ `string(user.ID)` — баг (int → string conversion)

✉️ 5. MESSAGE MODULE
📡 API
WebSocket /ws

Query:

?user_id=1&room_id=abc&token=valid
🧠 MessageService
Основной метод:
Process(msg)

Pipeline:

validate
rate limit
user check
room check
permission check
async:
save (Postgres)
broadcast (Kafka)
update activity
⚡ RateLimiter
Тип	Лимит
text	10/sec
command	2/sec
typing	5/sec
file	1/sec
⚠️ Проблемы
❌ нет retry для Kafka
❌ нет guaranteed delivery
⚠️ async без контроля ошибок
⚠️ возможна потеря сообщений
🌐 6. WEBSOCKET
Компоненты:
WebSocketServer
принимает соединения
валидирует (очень примитивно)
ConnectionManager
хранит:
users
rooms
broadcast
Client
read/write loop
⚠️ Проблемы
❌ token validation = token == "valid"
❌ нет heartbeat management
❌ readPump не обрабатывает сообщения (!)
❌ нет backpressure стратегии
💾 7. STORAGE
PostgreSQL (sqlc)
users
rooms
messages
2FA
Redis
cache:
rooms
users
sessions
temp tokens
Kafka
message distribution
FileStorage (cold storage)
Особенности:
батчинг сообщений
сегментация:
video → hourly
group → 4h
private → daily
⚠️ Проблемы
❌ FileStorage не используется (создается, но игнорируется)
❌ Redis errors не обрабатываются
⚠️ нет TTL стратегии глобально
🔌 8. API SUMMARY
AUTH
Method	Path
POST	/auth/verify
POST	/auth/enable
CHAT
Method	Path
POST	/chat/messages
POST	/chat/rooms
GET	/chat/rooms/:id
GET	/chat/rooms/:id/messages
WS

| GET | /ws |

🧩 9. НЕИСПОЛЬЗУЕМЫЕ / НЕЗАКОНЧЕННЫЕ УЧАСТКИ
❌ Auth
DisableTwoFA handler
email sender
❌ Chat
UpdateRoom/DeleteRoom/ListUserRooms (есть, но не используются)
❌ Message
FileStorage (не подключен)
Message batch save (не используется)
❌ WebSocket
readPump не вызывает service.Process
нет reconnection logic
❌ Security
JWT validation отсутствует в WS
нет refresh flow в WS
🚀 10. ФУНКЦИИ, КОТОРЫЕ МОЖНО БЫСТРО ДОБАВИТЬ
🔥 HIGH VALUE (быстро + мощно)
1. DisableTwoFA
уже почти готов
2. JWT validation в WebSocket
parse token → extract user_id
3. ListUserRooms API
storage уже есть
4. UpdateRoom API
сервис уже есть
5. Message retry (Kafka)
обернуть producer
6. Read receipts
тип уже есть (TypeReadReceipt)
7. Typing indicator
уже есть тип (TypeTyping)
8. File uploads
уже есть TypeFile + FileStorage
9. Presence system (online/offline)
почти готов (GetUserStatus)
10. Message pagination cursor-based
заменить limit/offset
⚠️ 11. КЛЮЧЕВЫЕ ПРОБЛЕМЫ АРХИТЕКТУРЫ
🚨 Критичные
❌ WebSocket небезопасен
❌ сообщения могут теряться
❌ нет транзакционности
❌ слабая валидация
⚠️ Средние
нет observability
нет metrics
нет tracing
нет retries
🟡 Minor
naming (RenovationTwoFA...)
inconsistent ID types (string/int)
🧭 12. РЕКОМЕНДАЦИИ
🔧 Архитектура
внедрить event sourcing light
добавить outbox pattern
🔐 Security
JWT middleware для WS
rate limit по IP
📊 Observability
Prometheus
OpenTelemetry
⚡ Performance
batching Kafka
connection pooling WS
📌 Итог

Твой проект — это уже:

✅ production-ready база
✅ real-time чат
✅ scalable через Kafka
✅ модульная архитектура

НО:

⚠️ требует доработки безопасности
⚠️ требует стабилизации messaging слоя
