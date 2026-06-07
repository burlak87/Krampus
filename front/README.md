# 🎄 KrampusMessage

[![Node.js](https://img.shields.io/badge/Node.js-20+-green)](https://nodejs.org/)
[![Go](https://img.shields.io/badge/Go-1.21+-blue)](https://golang.org/)

**KrampusMessage** – учебный мессенджер с поддержкой текстового чата и видеозвонков по технологии WebRTC. Проект демонстрирует интеграцию фронтенда на **Nuxt 3 (Vue 3, TypeScript)** и бэкенд-сервера сигналинга на **Go**.

> ⚠️ Статус: **Proof of Concept / активная разработка**. Основной функционал (обмен сообщениями, установка WebRTC-соединения) находится в стадии реализации.

---

## ✨ Возможности

- 📝 Текстовые сообщения в реальном времени (через WebSocket)
- 🎥 Видеозвонки peer‑to‑peer на базе WebRTC
- 🧭 Боковая панель навигации
- 🔧 Модульная архитектура с использованием Pinia (stores) и композаблов
- 🖥️ Сервер сигналинга на Go (WebSocket, обработка SDP/ICE)

---

## 🛠️ Технологический стек

| Компонент       | Технологии                                                                 |
|----------------|----------------------------------------------------------------------------|
| Клиент         | Nuxt 3, Vue 3, TypeScript, Pinia, WebRTC                                   |
| Стили          | CSS (глобальные стили + компонентные)                                      |
| Бэкенд (сигналинг) | Go, gorilla/websocket (или аналоги), стандартная библиотека             |
| Сборка фронта  | Vite (через Nuxt)                                                          |

---

## 📁 Структура проекта
```
KrampusMessage/
├── app/ # Клиентская часть (Nuxt)
│ ├── assets/ # SCSS/CSS, изображения
│ ├── components/ # Vue-компоненты (например, VideoChat, Chat)
│ ├── composables/ # Переиспользуемая логика (useWebRTC, useSocket)
│ ├── middleware/ # Роутер-гарды (например, auth)
│ ├── pages/ # Страницы приложения (index.vue, call.vue)
│ ├── stores/ # Pinia-сторы (user, messages, webrtc)
│ └── app.vue # Корневой компонент
├── server/
│ └── api/ # Go-сервер сигналинга
│   └── main.go # Точка входа, обработка WebSocket
├── schemas/ # TypeScript-интерфейсы (message.ts, user.ts)
├── types/ # Глобальные типы (signaling.ts)
├── public/ # Статика (favicon, robots.txt)
├── nuxt.config.ts # Конфигурация Nuxt
├── package.json # Зависимости фронтенда
├── tsconfig.json # Настройки TypeScript
└── .gitignore
```

## 🚀 Быстрый старт

### Требования

- [Node.js](https://nodejs.org/) (версия 20 или выше)
- [Go](https://golang.org/) (версия 1.21 или выше)
- Менеджер пакетов (npm, yarn или pnpm)

### 1. Клонирование репозитория

```
git clone https://github.com/Hahog/KrampusMessage.git
cd KrampusMessage
```

### 2. Запуск клиента (Nuxt 4)

```
npm run dev
```

# Установка зависимостей
```npm install```

# Режим разработки
```npm run dev
Клиент будет доступен по адресу: http://localhost:3000
```

### 3. Запуск сервера сигналинга (Go)
```
cd server/api
```

# Запуск сервера
```
go run main.go
```

