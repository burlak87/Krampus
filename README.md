# MAKE INTERNET GREAT AGIAN

Добавление Infisical в момент деплоя
Ключи шифрования
Индексация постов
Использование прокси
Политека конфидициальности
Пользовательское соглашение

Так значит теперь реализуем только эти функции, так же полными файлами для копирования, и упоминаниями, что, где и как:
chunk checksum validation
resume conflict resolution
session locking
chunk integrity verification
hash verification
tamper validation
transactional finalize
integrity verification
date ranges
attachments
mentions
reactions
validation
routing
join tokens
ack replay
offline restore
rebuild worker
repair jobs
moderation tooling
failover
ownership
lease ownership
heartbeat
rebalancing
multiple choice
poll closing
vote events
poll projections

Отличная практика, но не обязательно для работы приложения это сильно улучшает production-качество, масштабирование, UX, эксплуатацию, но приложение сможет жить и без этого.
- avatar versioning
- parallel upload coordination
- chunk deduplication
- media filters
- reaction filters
- mention filters
- polls
- deep link resolver
- sync projections
- folder unread counts
- folder ordering
- folder projections
- delta compaction
- compressed diffs
- pack permissions
- animated stickers
- cdn manifests
- emoji aliases
- emoji moderation
- emoji caching
- silent read projections
- ML scoring
- behavioral detection
- rebalance
- anonymous polls
- quiz mode

Не обязательно, это уже advanced enterprise/high-scale/large-platform уровень. Делать стоит только если реально появится нагрузка или продуктовая необходимость.
- semantic diffs
- link graph analysis
- custom partition ownership optimization
- advanced stream balancing

ЧТО МОЖНО ОТЛОЖИТЬ НАДОЛГО

Вот это вообще не мешает запуску production:

semantic diffs
ML scoring
link graph analysis
animated stickers
cdn manifests
emoji aliases


Проверь что ты уже создал из моего списка что я прислал в начале, а что нет и продолжай делать следуя ему, общайся со мной только на русском без англицизмов:
1. Профили пользователей и их настройки (отдельная доменная область, как auth). 2. Сервис загрузки файлов (полный функционал отправка, удаление, скачивание и загрузка). 3. Сжатие и шифрование сообщений, изображений и файлов (Чат данные сжимаются zstd, статичные данные сжимаются Brotli, профили и настройки пользователей сжатие LZ4, сжатие для изображений WebP, для видео h.265. Шифрование AES-256-GCM. Сначала сжатие, потом шифрование). 4. Поиск 5. Роли владелец администратор модератор участник гость 6. Матрица разрешений 7. Пригласительные ссылки 8. Запросы на вступление 9. Сохраненные сообщения Похоже на Telegram. 10. Удаление сообщений: мягкое удаление, жесткое удаление. 11. Редактирование сообщений, а имено: edited_at, edit_history 12. Ответы на сообщения 13. Темы сообщений 14. Пересылка сообщений 15. Закрепленные сообщения 16. Реакции и стикеры ❤️ 🔥 👀 16. Mentions. 17. Message scheduling 18. Moderation & Safety 19. Multi-device sync 20. Event sourcing 21. Media deduplication 22. Опросы.

так теперь давай работать по этому плану(тебе надо придерживаться пунктов, но внутри них можешь менять порядок и делать то что нужно). Так же не трогай ничего связанного с протоколами транспортировки сообщений (SSE, WebSocket) на данном этапе. Список:
1. Профили пользователей и настройки
- версии настроек
- optimistic locking
- аудит изменений
- аватары профиля
- privacy settings
- device settings
2. Сервис загрузки файлов
- multipart upload
- resumable upload
- signed urls
- virus scanning
- quotas
- retention policies
- lifecycle cleanup
3. Поиск
- ranking
- typo tolerance
- search projections
- indexing worker
- attachment search
- mention search
- filters
- pagination optimization
4. Пригласительные ссылки
- max uses
- revocation
- invite audit
- invite permissions
- deep links
5. Запросы на вступление
-  approval workflow
- moderation queue
- notifications
- expiration
6. Сохраненные сообщения
7. Редактирование сообщений
- edit_history
- diff storage
- edit events
8. Темы сообщений
- topic projections
- unread per topic
- topic permissions
9. Пересылка сообщений
- forward chain
- protected forwards
- attribution validation
10. Закрепленные сообщения
- pin ordering
- multiple pins
- pin events
11. Реакции и стикеры
- aggregation
- realtime sync
- sticker packs
- custom emoji
- counters
12. Moderation & Safety
- bans
- mute system
- shadow bans
- anti spam
- rate limiting
- abuse detection
- media moderation
- audit logs
13. Multi-device sync
- cursor sync
- ack system
- delta sync
- websocket sync
- offline replay
14. Event sourcing
- projections
- snapshots
- replay
- partitioning
- consumers
- event versioning
15. Опросы



Общая архитектура авторизации

!
Переименовать все нужные пакеты(кроме переиспользуемых из pkg) в формат auth-”название пакета”.
Пересмотреть генерацию рандомных кодов и генерацию хэша. А так же  middleware для проверок.
Нужно добавить поиск по имени пользователя(@username)
Создать функцию отправки сообщения.
Создать функции(Генерация, получение, новый/обновление) для подтверждение почты, двухфакторной аутентификации и сброса пароля.
Создание функций helper’ов.
Добавить отправку кода на почту, добавить проверку почты. Добавить Passkey, как код из цифр и как код из симфолов. С возможностью создать свой или рандомный.
Создать кэширование в Redis, может токены перенести в формат сессии там же. Написать сессии, которые будут вызываться в большинстве методов. Они должны сохраняться в Redis. В сессии будет храниться айди, по которому мы после будет совершать поиск. Разработать поиск по сессии и по бд в сущности пользователь
Создать декоратор для ролей пользователей. с ключами ролей. Так же создадим guard для ролей. Создать декоратор или миддлевару для проверки ролей в нужных местах.
Создать Провайдеры, Политики и другие вспомогательные классы?
Мьютекс для базы данных, чтобы одновременно не долбились клиенты. Защита от рейс кондишена. 
Объявить часть ошибок.
Разделить handler, service, repository.
В метод регистерроудс передается мультиплексор (mux) для регистрации обработчиков. Тоесть сначала регистрируетсся мультиплексор, после передается в функцию и регистриуются маршруты в этой функции.
У реквеста использовать метод PathValue для получения передаваемых данных в пути, к примеру айдишнику для удаления. В таком случае сразу надо перевести в нужный тип данных и обработь на случай ошибки, вдруг будет буква. 
Обработка ошибок заданных в приложении для клиентов и пользователей в handler. Для этого используется errors.Is(err, name_variable_error).
Валидация отдельная история.
Нельзя делать так чтобы списалось больше, чем есть на самом деле.
Понятные сообщения об ошибках.
Бизнес-логика должна быть тестируемой без HTTP.
Проверять является ли ручка полезной или безполезной.
Структурная валидация в Handler, к примеру не равно нулю, меньше нуля, а далее проверяем что всего хватает и мы реализуем методы для получения и другие, главное следить за тем чтобы не обращался напрямую к services, не делал бизнес-валидаацию, не было бизнес-логики и не знал про внутреннюю структуру сервиса. Структурная валидация это когда мы проверяем верность введенных данных. Бизнес-валидация это когда нужно к примеру в базу сходить. Их по хорошему нужно разделять, так как первая завяязана на транспортном слое, а вторая на бизнес слое. 
Отдельно вынести ошибки в переменные в отдельном пакете.
Интефейсы для тестирования и продакшена, как контракт на методы. Позволяет создавать фиктивные заклушки более нижнего слоя для структур используемых в unit-тестах. Автоматическая генерация mock для тестов.
Принимаем интерфейсные типы, а возвращаем конкретные.
Важна атомарность.
Проверять есть ли свободное, которое можно зарезервировать.
Вынести роуты в отдельный файл.
Проверять на несколько возможных кастомных ошибок приложения можно в конструкции свич кейс где кейс равен errors.Is(err, name_variable_error).
Атомарность можно реализовать через мьютекс, но это плохое решение.
Создать роутер и отдельные папки в handler которые приватные структуры подтягивают через интерфейс.
Проверки совместимости.
Правила совместимости должны быть в коде.
Богатая доменная модель, правила относятся к доменам и там же должны хранится в методах. В сервисном слое уже остаются методы для маршрутизирования и синхранизации.
Value Onject - это объект важный своим содержимым, а не тем что он уникальный или интересный. У него нет ID, если есть 2 объекта с одинаковыми полями они считаются одним и тем же. Являются неизменяемыми. Примеры: цвет, деньги. 

Богатая доменная модель(DDD), Value Objects, Entity, Aggregate, DDD+System Design, DTO, Record, Domain.

2.30 часы.

task file. golang ci ling

Redis для токенов, как jwt, так и сессий, а на фронте хроняться в куки с влажком httpOnly.

Добавить Profile. 
Интегрировать Secret_Key для Google reCAPTCHA полученный в админ панели сайта при создании нового проекта. Там нужно будет скопировать два ключа, один для сервера, второй для клиента. 
Создать авторизацию через социальные сети, а именно OAuth2.0. Надо выбрать писать самому или использовать библиотеку.
Настроить почтовый сервер для отправки email-писем, а именно подтверждение почты, двухфакторной аутентификации и сброса пароля. Можно через resend.ru. 

Добавить Чат-ботов, Инлайн ботов, ботов с мини-апп.
Добавить бизнес-модули


Интересное:
role UserRole @default(REGULAR)
displayName string
picture string

isVerified Boolean @default(false) @map("is_verified")
idTwoFactorEnabled Boolean @default(false) @map("is_two_factor_enabled")

method AuthMethod

accounts Account[]

enum UserRole {
  REGULAR
  ADMIN
}

enum AuthMethod {
  CREDENTIALS
  GOOGLE
  YANDEX  
}

enum TokenType {
  VERIFICATION
  TWO_FACTOR
  PASSWORD_RESET
}

Account

type string
provider string
refreshToken string?
accessToken string?
expiresAt Int
user User?
userId string?

Token

email string
token string
type TokenType
expiresIn DateTime

🚀 KRAMPUS — ROADMAP ДО УРОВНЯ SENIOR / STAFF ENGINEERING
1. ГЛАВНЫЕ ПРОБЛЕМЫ ТЕКУЩЕЙ АРХИТЕКТУРЫ

Сейчас проект выглядит как: `Strong Mid-Level Realtime Backend`
Чтобы поднять его до уровня: `Senior / Staff / Production-grade Distributed Platform`

нужно решить 5 ключевых классов проблем:
Категория	                  Статус
Security	                  weak
Reliability	                medium
Observability	              missing
Scalability	                medium
Maintainability	            medium

2. SENIOR/STAFF REFACTOR ROADMAP
PHASE 1 — CRITICAL REFACTORING

2.1 Убрать fake websocket auth
Сейчас `if token != "valid"`
Нужно
JWT middleware для WS
1. Parse JWT
2. Validate signature
3. Validate expiration
4. Validate session
5. Validate room membership

2.2 Вынести authentication middleware
Сейчас

Логика auth размазана.
Нужно:
```
pkg/auth/
├── jwt/
├── middleware/
├── session/
└── permissions/
```

2.3 Исправить inconsistent IDs
Сейчас `string(user.ID)`
Нужно
Единый тип:
```
type UserID string
type RoomID string
type MessageID string
```

2.4 Ввести Context Propagation
Сейчас

Часть goroutine теряет context.

Нужно
```
trace_id
request_id
user_id
correlation_id
```

во всех слоях.

2.5 Переписать async goroutines
Сейчас
```
go save()
go publish()
```

Проблемы:
- 🚨 panic loss
- 🚨 no retries
- 🚨 no supervision
- 🚨 race conditions

Нужно
Worker Pool `pkg/workerpool/`

2.6 Ввести structured logging
Сейчас `log.Printf(...)`
Нужно
```
{
  "level": "error",
  "trace_id": "...",
  "user_id": "...",
  "room_id": "...",
  "msg": "..."
}
```

2.7 Удалить business logic из adapters
Сейчас
handlers partially contain logic.

Нужно `adapters → only transport mapping`

2.8 Ввести interfaces everywhere
Сейчас `storage *RoomPGStorage`
Нужно `storage RoomRepository`

2.9 Ввести dependency injection
Сейчас `Manual wiring в main.go.`

Нужно
```
uber/fx
google/wire
```
или собственный container.

2.10 Разделить bounded contexts
Сейчас `user/chat/message tightly coupled.`
Нужно
```
identity
realtime
messaging
presence
media
notification
```

PHASE 2 — RELIABILITY
3.1 Outbox Pattern
Сейчас `DB save и Kafka publish не атомарны.`

Проблема
```
message saved
BUT kafka failed
```

Нужно
```
DB Transaction
 ↓
Outbox table
 ↓
Reliable publisher
```

3.2 Delivery ACK System
Нужно
```
sent
delivered
read
failed
```

3.3 Retry System
Нужно
```
retry queue
exponential backoff
DLQ
```

3.4 Idempotency
Сейчас `duplicate messages possible.`
Нужно
```
message_id uniqueness
dedup cache
```

3.5 Distributed Rate Limiter
Сейчас `in-memory limiter.`

Проблема не работает при scaling.

Нужно Redis token bucket.

PHASE 3 — STORAGE REFACTOR
4.1 Normalized Room Members
Сейчас
```
members JSON
```

Нужно
```
room_members
```

4.2 Message Partitioning
Нужно

```
Postgres partitions:
- by month
- by room shard

4.3 Redis Cache Strategy
Нужно
```
cache-aside
write-through
cache invalidation bus
```

4.4 FileStorage → Object Storage
Нужно `S3 / MinIO`

4.5 Search Index
Нужно
`OpenSearch / Elasticsearch`

PHASE 4 — OBSERVABILITY
5.1 OpenTelemetry
Нужно
```
traces
spans
metrics
```

5.2 Prometheus Metrics
Нужно
```
Metric
ws_connections
messages/sec
kafka_latency
db_latency
```

5.3 Grafana Dashboards
Нужно
```
realtime traffic
room activity
kafka lag
online users
```

5.4 Distributed Tracing
Нужно
`REST → Kafka → WS`

PHASE 5 — REALTIME ENGINE
6.1 WebSocket Hub Refactor
Сейчас `sync.Map everywhere.`

Нужно `Actor-like architecture.`

6.2 Presence Engine
Нужно
```
online
away
busy
invisible
mobile
desktop
```

6.3 Session Recovery
Нужно `resume websocket sessions`

6.4 SSE Transport
Нужно `fallback transport.`

6.5 Event Stream
Нужно `event sourcing light`

PHASE 6 — STAFF-LEVEL ARCHITECTURE
7.1 CQRS
Нужно 
```
write model
read model
```

7.2 Event Sourcing
Нужно `events вместо state mutation.`

7.3 Saga Orchestration
Нужно для:
```
calls
payments
moderation
notifications
```

7.4 Multi-region readiness
Нужно `geo replication`

7.5 Federation
Нужно `cross-server chats.`

3. ФУНКЦИИ ДЛЯ KRAMPUS CHAT
CORE CHAT FEATURES
1. Message Editing
```
edited_at
edit_history
```
2. Message Delete
```
soft delete
hard delete
moderation delete
```
3. Read Receipts `seen by`
4. Typing Indicators
5. Replies / Threads
6. Reactions `👍 ❤️ 🔥 👀`
7. Pinned Messages
8. Forward Messages
9. Saved Messages `Telegram-like.`
10. Drafts `REALTIME FEATURES`
11. Presence System
12. Voice Channels `Discord-like.`
13. Screen Sharing
14. Live Streaming
15. Watch Together `YouTube/Twitch sync.`

GROUP FEATURES
16. Roles
```owner
admin
moderator
member
guest```
17. Permissions Matrix
18. Invite Links
19. Join Requests
20. Group Verification

AI FEATURES
21. AI Moderation
22. AI Summaries
23. Semantic Search
24. AI Assistant Bot
25. Auto Translation

ENTERPRISE FEATURES
26. Audit Logs
27. Retention Policies
28. Compliance Mode
29. DLP
30. SSO/OAuth

MEDIA FEATURES
31. File Upload Service
32. CDN Integration
33. Image Compression
34. Video Transcoding
35. Voice Notes

SECURITY FEATURES
36. Device Management
37. Session Management
38. Login Alerts
39. Encrypted Chats
40. Secret Chats

GAMING FEATURES
41. Rich Presence
42. Matchmaking Chat
43. Party Voice Rooms
44. Game Activity Status
45. Overlay Integration

SOCIAL FEATURES
46. Stories
47. Profiles
48. Friend System
49. Follow System
50. Public Communities

STAFF-LEVEL FEATURES
51. Distributed Presence
52. Realtime Analytics
53. Hot Partition Protection
54. Dynamic Sharding
55. Adaptive Rate Limiting
56. Intelligent Caching
57. Predictive Prefetching
58. Priority Messaging
59. QoS Messaging
60. Offline Sync Engine

4. ИДЕАЛЬНАЯ ЦЕЛЕВАЯ АРХИТЕКТУРА KRAMPUS
```Clients
  ↓
Gateway
  ↓
Realtime Engine
  ↓
Event Bus (Kafka)
  ↓
Messaging Core
  ↓
  Storage Layer```

5. ИТОГ
Сейчас Krampus:
- хороший modular monolith
- strong realtime prototype
- mid+/senior-level foundation
После roadmap:
- distributed realtime platform
- enterprise-grade messenger backend
- Discord/Slack/Telegram-class architecture foundation
- staff-level engineering system

Вот что действительно ценного и нового можно вытащить из этих старых файлов для текущего Krampus — именно то, чего у тебя либо вообще нет, либо реализовано частично/слабо.

🚨 1. ГЛАВНОЕ: ТВОЯ СТАРАЯ АРХИТЕКТУРА БЫЛА БОЛЕЕ STAFF-LEVEL

Самое важное:
год назад ты уже думал как:
- highload engineer
- distributed systems engineer
- realtime architect

В parse.md часть этих идей:
- потерялась
- не реализована
- упростилась

🧠 2. ЧТО НУЖНО ВЕРНУТЬ В KRAMPUS
2.1 MESSAGE-CENTRIC DESIGN

Это одно из лучших решений из старых файлов.
Идея

ВСЯ система строится вокруг:
```
type BaseMessage struct {
    ID        string
    Type      MessageType
    RoomID    string
    UserID    string
    Timestamp time.Time
    Payload   any
}
```

Почему это очень важно

Сейчас у тебя: `transport-centric architecture`
Нужно: `message-centric architecture` 
Что это дает
```Единый pipeline
REST
WS
SSE
Kafka
Storage
→ работают с одной DTO
```

Это позволит:
✅ retries
✅ DLQ
✅ batching
✅ routing
✅ versioning
✅ event sourcing
✅ analytics
✅ replay

2.2 SSE КАК ОСНОВНОЙ TRANSPORT

Это было очень сильной идеей.
Сейчас
У тебя: `WebSocket-first`
Нужно 
```
SSE-first
WS-second
```

Почему SSE лучше
SSE	                     WS
дешевле	                 дороже
проще scaling	           sticky sessions
auto reconnect	         manual reconnect
HTTP infra friendly	     stateful
меньше RAM	             больше RAM

Идеальная схема
SSE Использовать для:
- receiving messages
- notifications
- presence
- room updates
WebSocket Только для:
- typing
- voice
- realtime collaboration
- games
- calls

Это очень staff-level решение
Discord/Slack-like systems делают похожее.

2.3 SHARDED CONNECTION MANAGER
Это ОЧЕНЬ важно. Сейчас У тебя:
`sync.Map`

Проблема После: `10k+ connections` получишь:
```
contention
GC pressure
lock bottlenecks
```

Нужно
`64 shards`

Архитектура
```
ConnectionManager
    ↓
Shards[64]
    ↓
User Connections
```
Это даст
✅ меньше contention
✅ better CPU cache locality
✅ linear scaling
✅ predictable latency

Это production pattern Tinode / Discord-like.

2.4 MULTI-DEVICE SESSION ENGINE

Сейчас у тебя:
1 user = 1 connection
Нужно:
1 user = N sessions
Возможности
mobile
desktop
browser
tablet
Что хранить
type Session struct {
    DeviceType string
    LastSeen   time.Time
    UserAgent  string
    Geo        string
}
Это база для:

✅ push notifications
✅ session management
✅ device logout
✅ security alerts

2.5 PRIORITY MESSAGE PIPELINE

Очень хорошая идея.

Сейчас

Все сообщения одинаковые.

Нужно
Priority	Type
HIGH	moderation/system
NORMAL	text
LOW	typing/read
Поведение
HIGH
instant flush
NORMAL
batch by 50
timeout 100ms
LOW
aggressive batching
Это dramatically снизит:

✅ disk IO
✅ Kafka pressure
✅ CPU usage
