# 📘 KRAMPUS — ПОЛНАЯ ТЕХНИЧЕСКАЯ ДОКУМЕНТАЦИЯ
1. Архитектура проекта
1.1 Тип архитектуры

Проект построен как: `Modular Monolith + Event Driven Messaging`

Архитектура сочетает:
- модульный монолит
- realtime-коммуникации
- асинхронную обработку через Kafka
- кэширование через Redis
- persistence layer через PostgreSQL
- файловый cold storage

1.2 Общая схема
                ┌────────────────────┐
                │      REST API      │
                │       (Gin)        │
                └─────────┬──────────┘
                          │
                ┌─────────▼──────────┐
                │    Service Layer   │
                └─────────┬──────────┘
                          │
        ┌─────────────────┼─────────────────┐
        │                 │                 │
        ▼                 ▼                 ▼
 PostgreSQL            Redis             Kafka
 Persistence          Cache            Distribution
        │                                   │
        └──────────────► WebSocket ◄────────┘
                          Manager
                             │
                    Connected Clients


1.3 Структура модулей

Все модули организованы одинаково:
```
internal/<module>/
├── adapters/   → REST / WS handlers
├── domain/     → entities / models
├── service/    → business logic
└── storage/    → db/cache layer
```

2. MODULE: AUTH
2.1 Назначение

Модуль отвечает за:
- 2FA
- access token
- refresh token
- temp token
- verification flow

2.2 Структура
```
internal/auth/
├── adapters/
├── domain/
├── service/
└── storage/
```

2.3 REST API
`POST /api/v1/auth/verify`
Назначение: Проверка 2FA кода.
```
Request
{
  "code": "123456",
  "temp_token": "jwt-temp-token"
}
```

```
Response Success
{
  "access_token": "jwt-access-token",
  "refresh_token": "jwt-refresh-token"
}
```

```
Response Error
{
  "error": "Verification failed"
}
```

Возможные ошибки:
Ошибка	                  Причина
invalid temp token	      JWT невалиден
code expired	            код истек
too many attempts	        превышен лимит
code already used	        повторное использование


`POST /api/v1/auth/enable`
Назначение: Включение двухфакторной авторизации.
```
Headers
Authorization: Bearer JWT
```

```
Request
{}
```

```
Response
{
  "success": true
}
```

2.4 Service Layer
Service: `TwoFA`

Method: `EnableTwoFA(userID int64)`
Логика:
1. вызывает storage
2. обновляет поле two_fa_enabled

Method: `UsersSendEmailCode(tempToken string)`
Полный pipeline:
1. Проверка Redis temp token
2. Decode JWT
3. Проверка rate-limit
4. Генерация 6-digit code
5. Сохранение в PostgreSQL
6. Отправка email

Rate Limits:
Ограничение	     Значение
code requests	   3 / 15 мин

Недоработки:
- ❌ email sender = stub
- ❌ нет SMTP
- ❌ нет retry queue
- ❌ нет anti-spam
- ❌ нет blacklist IP

Method: `VerifyCode(code user.Code)`
Pipeline:
1. Decode temp token
2. Проверка recent attempts
3. Получение кода из DB
4. Проверка attempts
5. Проверка expiration
6. Проверка совпадения
7. Mark used
8. Generate JWT

Rate Limits:
Ограничение	       Значение
verify attempts	   5 / 10 мин
code attempts	     3

2.5 Storage Layer
PostgreSQL Storage
Файл: `internal/auth/storage/psqlTwofaStorage.go`

Методы:
`RenovationTwoFAStatus`
```
UPDATE users SET two_fa_enabled = true
```

`InsertTwoFaCode`
Сохраняет:
- code
- expires_at
- user_id

`SelectTwoFaCodeByUserID`
Возвращает:
- latest code
- attempts
- expiration

`MarkTwoFaCodeUsed`
```
UPDATE two_fa_codes SET is_used = true
```

Проблемы слоя:
- ⚠️ название `Renovation*` некорректно
- ⚠️ нет транзакций
- ⚠️ нет optimistic locking
- ⚠️ race conditions возможны

3. MODULE: CHAT
3.1 Назначение

Модуль управляет:
- комнатами
- участниками
- permissions
- room metadata

3.2 REST API
`POST /api/v1/chat/rooms`
Назначение: Создание комнаты
```
Request
{
  "id": "room-1",
  "type": "group",
  "name": "Developers",
  "members": ["1", "2"]
}
```

```
Response
{
  "room": {
    "id": "room-1",
    "type": "group",
    "owner_id": "1",
    "members": ["1", "2"]
  }
}
```

`GET /api/v1/chat/rooms/:room_id`
Назначение: 
```
Response
{
  "id": "room-1",
  "type": "group",
  "owner_id": "1"
}
```

`GET /api/v1/chat/rooms/:room_id/messages`
```
Query
?limit=50
```

```
Response
{
  "messages": []
}
```

`POST /api/v1/chat/messages`
Назначение: 
```
Request
{
  "type": "text",
  "room_id": "room-1",
  "payload": {
    "text": "hello"
  }
}
```

```
Response
{
  "status": "sent",
  "msg_id": "uuid"
}
```

3.3 Service Layer
Service: `RoomService`

Method: `GetRoom`
Pipeline:
1. Redis cache
2. PostgreSQL fallback
3. Cache warmup

Method: `CanSendMessage`
Проверяет: 
- membership
- room type
- readonly
- file permissions
Rules:
Room Type	           Rule
personal	           owner only
private	all          members
group	all            members
video_call	         active call only

Недоработки:
- ❌ ACL отсутствует
- ❌ roles отсутствуют
- ❌ moderation отсутствует

Method: `CreateRoom`
Pipeline:
1. validate
2. timestamps
3. save
4. cache

Method: `validateRoom`
Проверяет:
- empty ID
- owner
- room type

Нереализованные методы:
Метод	                   Статус
UpdateRoom	             partially
DeleteRoom	             partially
ListUserRooms	           partially

3.4 Storage Layer
PostgreSQL

Файл: `roomStorage.go`

Особенности:
Members - Сохраняются как JSON:
["1","2","3"]

Settings - Тоже JSON:
```
{
  "allow_files": true
}
```

Методы:
Метод	          Назначение
SaveRoom	      UPSERT
GetRoom	        SELECT
UpdateRoom	    UPDATE
DeleteRoom	    DELETE
ListUserRooms	  JSON contains query

Недостатки:
- ⚠️ JSON search медленный
- ⚠️ нет indexes для members
- ⚠️ нет room_members table

Redis Cache
Ключи: `room:<id>`
TTL: `10 minutes`

Проблемы:
- ❌ cache invalidation incomplete
- ❌ no distributed cache consistency

4. MODULE: MESSAGE
4.1 Назначение

Модуль отвечает за:
- сообщения
- realtime
- kafka distribution
- websocket
- file persistence

4.2 REST TRANSACTIONAL LAYER
`POST /chat/messages`
```
Транзакционный pipeline
REST
 ↓
Gin Handler
 ↓
MessageService.Process
 ↓
Validation
 ↓
RateLimiter
 ↓
Room Validation
 ↓
Async Save
 ↓
Kafka Broadcast
 ↓
WebSocket Broadcast
```

4.3 WEBSOCKET TRANSACTIONAL LAYER
Endpoint: `/ws`

Connection Flow:
1. Upgrade HTTP
2. Validate params
3. Register connection
4. Subscribe room
5. Start pumps

Query Params: `/ws?user_id=1&room_id=abc&token=valid`
```
Incoming WS Message
{
  "type": "text",
  "payload": {
    "text": "hello"
  }
}
```

```
Outgoing WS Message
{
  "id": "uuid",
  "type": "text",
  "user_id": "1"
}
```

Проблемы WS: 
- 🚨 token validation fake
- 🚨 no reconnect strategy
- 🚨 no heartbeat cleanup
- 🚨 no session recovery

4.4 SSE Layer
Статус: ❌ НЕ РЕАЛИЗОВАН

Возможное применение:
- notifications
- typing events
- room events
- system events

4.5 Service Layer
MessageService
Method: `Process`
Главный transactional orchestrator.

Полный pipeline:
1. Timestamp
2. Validation
3. Rate limit
4. User lookup
5. Room lookup
6. Permission check
7. Async DB save
8. Async Kafka publish
9. Activity update

Возможные ошибки:
Ошибка	                Причина
invalid message	        validation
room not found	        room missing
forbidden	              permission
rate exceeded	          limiter

Недостатки:
- 🚨 goroutines без supervision
- 🚨 потеря сообщений возможна
- 🚨 no retries
- 🚨 no dead-letter queue

Типы RateLimiter:
Type	          Limit
text	          10/sec
command	        2/sec
typing	        5/sec
file	          1/sec

Cleanup - Удаляет limiter через:
`10 minutes inactivity`

4.6 PostgreSQL Message Storage
`SaveMessage`
Сохраняет:
- metadata
- payload
- timestamps

`SaveMessageBatch`
Подготовлен для batching.

Статус: ⚠️ практически не используется

4.7 Kafka Layer
MessageDistributor
Назначение: Асинхронное распределение сообщений.
```
Kafka Message
{
  "id": "uuid",
  "room_id": "room-1",
  "payload": {}
}
```

Pipeline:
```
MessageService
 ↓
Distributor
 ↓
Kafka Producer
 ↓
Kafka Topic
 ↓
Consumer
 ↓
WebSocket Broadcast
```

Kafka Config:
```
acks=all
idempotence=true
```

Проблемы: 
- 🚨 no retry policy
- 🚨 no partition strategy
- 🚨 no DLQ
- 🚨 no consumer rebalance handling

4.8 FileStorage Layer
Назначение: Cold storage / archival storage сообщений.

Pipeline:
```
Message
 ↓
Room Buffer
 ↓
Flush Strategy
 ↓
Segment File
```

Буферизация:
Немедленный flush:
- Type
- system
- command

Delayed flush:
Type	             Rule
text	             64KB / 100ms
typing	           500ms

Структура файлов:
Group rooms `/groups/<room>/<date>.log`
Video calls `/video_calls/<room>/<hour>.log`
Private chats `/private/<shard>/<room>.log`
Формат записи `timestamp|id|type|user|room|payload`

Проблемы:
- 🚨 file rotation incomplete
- 🚨 no cleanup policy
- 🚨 no compression
- 🚨 no encryption
- 🚨 no indexing

4.9 Redis Layer
User Cache `user_client:<id>`
TTL: `5 min`

Room Cache `room:<id>`
TTL: `10 min`

Проблемы:
- ⚠️ cache stampede possible
- ⚠️ no distributed invalidation
- ⚠️ no persistence guarantees

5. НЕ РЕАЛИЗОВАННЫЕ ФУНКЦИИ
Функция	                Статус
DisableTwoFA	          TODO
SSE	                    TODO
JWT WS validation	      TODO
Delivery ACK	          TODO
Retry queue	            TODO
Presence sync	          TODO
Read receipts	          partial
Typing sync	            partial
File uploads	          partial
Message batching	      partial

6. АРХИТЕКТУРНЫЕ ПРОБЛЕМЫ

Критичные: 
- 🚨 fake websocket auth
- 🚨 async message loss
- 🚨 no transactions
- 🚨 no guaranteed delivery

Средние:
- ⚠️ no observability
- ⚠️ no metrics
- ⚠️ no tracing
- ⚠️ no retries

Минорные:
- ⚠️ naming inconsistencies
- ⚠️ mixed string/int IDs
- ⚠️ missing interfaces

7. СПИСОК УЛУЧШЕНИЙ (СНОСКИ)
Security:
1. JWT validation for WS
2. refresh token rotation
3. IP rate limiting
4. CSRF protection
5. audit logging

Messaging:
1. Kafka retry queue
2. dead letter queue
3. delivery ACK
4. message deduplication
5. exactly-once pipeline

Storage:
1. room_members table
2. message partitioning
3. redis clustering
4. file compression
5. object storage migration

Realtime:
1. SSE support
2. reconnect recovery
3. websocket heartbeat
4. presence sync
5. typing sync

Architecture:
1. outbox pattern
2. CQRS
3. event sourcing
4. service interfaces
5. dependency injection container

Observability:
1. Prometheus
2. Grafana
3. OpenTelemetry
4. distributed tracing
5. structured logging

8. ИТОГОВАЯ ОЦЕНКА ПРОЕКТА
Сильные стороны:
✅ хорошая модульность
✅ realtime архитектура
✅ Kafka integration
✅ Redis cache layer
✅ продуман FileStorage
✅ separation of concerns

Слабые стороны:
⚠️ безопасность
⚠️ отсутствие reliability layer
⚠️ incomplete transactional guarantees
⚠️ partial implementations

Общая зрелость:
Компонент	         Оценка
Architecture 	     8/10
Realtime	         7/10
Security	         4/10
Reliability	       5/10
Scalability	       8/10
Maintainability	   7/10

Проект уже выглядит как:
1. production-grade backend foundation
2. scalable realtime platform
3. основа для messenger / collaboration platform / gaming chat backend

Но ему нужен:
1. hardening
2. reliability
3. observability
4. security layer
5. transactional guarantees
