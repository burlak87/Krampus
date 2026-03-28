-- Сообщения (основная таблица)
CREATE TABLE IF NOT EXISTS messages (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  user_id TEXT NOT NULL,
  room_id TEXT NOT NULL,
  timestamp BIGINT NOT NULL,
  payload JSONB NOT NULL,
  signature TEXT,
  created_at TIMESTAMP DEFAULT NOW(),
  expires_at TIMESTAMP
);
CREATE INDEX CONCURRENTLY idx_messages_room_timestamp ON messages(room_id, timestamp DESC);
CREATE INDEX CONCURRENTLY idx_messages_user_timestamp ON messages(user_id, timestamp DESC);

-- Комнаты
CREATE TABLE IF NOT EXISTS rooms (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  owner_id TEXT NOT NULL,
  name TEXT,
  members JSONB NOT NULL,
  settings JSONB,
  created_at BIGINT NOT NULL,
  updated_at BIGINT NOT NULL
);

-- Пользователи
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  email TEXT UNIQUE,
  status TEXT DEFAULT 'offline',
  last_active BIGINT,
  created_at BIGINT NOT NULL,
  permissions JSONB DEFAULT '[]',
  metadata JSONB
);
