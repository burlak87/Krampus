// type FileStorage struct {
// 	basePath    string
// 	segmentSize time.Duration
// 	buffers     sync.Map // roomID -> *RoomBuffer
// }

// func New(basePath string, segmentSize time.Duration) *FileStorage {
// 	os.MkdirAll(basePath, 0755)
// 	return &FileStorage{basePath: basePath, segmentSize: segmentSize}
// }

// func (f *FileStorage) SaveMessage(msg *domain.BaseMessage) {
// 	bufferI, _ := f.buffers.LoadOrStore(msg.RoomID, &RoomBuffer{})
// 	buffer := bufferI.(*RoomBuffer)
// 	buffer.mu.Lock()
// 	defer buffer.mu.Unlock()

// 	buffer.messages = append(buffer.messages, msg)
// 	// size calc + flush if TypeSystem/Command or size>64MB or time>100ms
// 	if shouldFlush(buffer, msg) {
// 		f.flushBuffer(buffer)
// 	}
// }

type FileStorage struct {
	basePath    string
	segmentSize time.Duration
	buffers     sync.Map // roomID → *RoomBuffer
}

type RoomBuffer struct {
	mu         sync.Mutex
	messages   []*domain.BaseMessage
	size       int64
	lastFlush  time.Time
	activeFile *os.File
	writer     *bufio.Writer
}

func New(basePath string, segmentSize time.Duration) *FileStorage {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		log.Printf("Failed to create base directory: %v", err)
	}
	return &FileStorage{
		basePath:    basePath,
		segmentSize: segmentSize,
	}
}

func (f *FileStorage) SaveMessage(roomID string, msg *domain.BaseMessage) error {
	buffer := f.getOrCreateBuffer(roomID)
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	buffer.messages = append(buffer.messages, msg)
	messageSize := int64(len(msg.Payload) + 100) // + метаданные

	// 🔥 УМНАЯ СТРАТЕГИЯ FLUSH
	shouldFlush := false
	switch msg.Type {
	case domain.TypeSystem, domain.TypeCommand:
		shouldFlush = true // немедленная запись

	case domain.TypeText, domain.TypeFile:
		buffer.size += messageSize
		shouldFlush = buffer.size >= 64*1024 || time.Since(buffer.lastFlush) > 100*time.Millisecond

	case domain.TypeTyping, domain.TypeReadReceipt:
		shouldFlush = time.Since(buffer.lastFlush) > 500*time.Millisecond

	default:
		shouldFlush = len(buffer.messages) >= 50
	}

	if shouldFlush {
		return f.flushBuffer(roomID, buffer)
	}
	return nil
}

func (f *FileStorage) getOrCreateBuffer(roomID string) *RoomBuffer {
	actual, _ := f.buffers.LoadOrStore(roomID, &RoomBuffer{
		messages:  make([]*domain.BaseMessage, 0),
		lastFlush: time.Now(),
	})
	return actual.(*RoomBuffer)
}

func (f *FileStorage) flushBuffer(roomID string, buffer *RoomBuffer) error {
	if len(buffer.messages) == 0 {
		return nil
	}

	if err := f.ensureFile(roomID, buffer); err != nil {
		return err
	}

	// 📝 Запись всех сообщений
	for _, msg := range buffer.messages {
		line := f.formatMessageLine(msg)
		if _, err := buffer.writer.WriteString(line); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
	}

	if err := buffer.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffer: %w", err)
	}

	if err := buffer.activeFile.Sync(); err != nil {
		log.Printf("Failed to sync file: %v", err)
	}

	// 🧹 Очистка буфера
	buffer.messages = buffer.messages[:0]
	buffer.size = 0
	buffer.lastFlush = time.Now()

	return nil
}

func (f *FileStorage) ensureFile(roomID string, buffer *RoomBuffer) error {
	now := time.Now()
	filePath := f.getSegmentPath(roomID, now)

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	buffer.activeFile = file
	buffer.writer = bufio.NewWriterSize(file, 64*1024) // 64KB буфер
	return nil
}

// 🗂️ УМНАЯ СЕГМЕНТАЦИЯ ПО ТИПАМ КОМНАТ
func (f *FileStorage) getSegmentPath(roomID string, t time.Time) string {
	roomType := f.getRoomType(roomID)

	switch roomType {
	case domain.RoomTypeVideoCall:
		// Видеозвонки: 1ч сегменты
		return filepath.Join(f.basePath, "video_calls", roomID,
			t.Format("2006-01-02"), fmt.Sprintf("%d.log", t.Hour()))

	case domain.RoomTypeGroup:
		// Групповые: 4ч сегменты
		hour := (t.Hour() / 4) * 4
		return filepath.Join(f.basePath, "groups", roomID,
			t.Format("2006-01"), fmt.Sprintf("%s_%02d.log", t.Format("2006-01-02"), hour))

	case domain.RoomTypePrivate:
		// Личные: 1 день + шардинг
		shard := roomID[:2]
		return filepath.Join(f.basePath, "private", shard, roomID,
			t.Format("2006-01-02")+".log")

	case domain.RoomTypePersonal:
		// Заметки: 1 месяц + шардинг
		shard := roomID[:2]
		return filepath.Join(f.basePath, "personal", shard, roomID,
			t.Format("2006-01")+".log")

	default:
		return filepath.Join(f.basePath, "default", roomID,
			t.Format("2006-01-02")+".log")
	}
}

func (f *FileStorage) formatMessageLine(msg *domain.BaseMessage) string {
	return fmt.Sprintf("%d|%s|%s|%s|%s|%s\n",
		msg.Timestamp, msg.ID, msg.Type, msg.UserID, msg.RoomID, string(msg.Payload))
}

// type FileStorage struct {
//   basePath    string
//   segmentSize time.Duration
//   buffers     sync.Map  // roomID → *RoomBuffer
// }

// type RoomBuffer struct {
//   mu         sync.Mutex
//   messages   []*domain.BaseMessage
//   size       int64
//   lastFlush  time.Time
//   activeFile *os.File
//   writer     *bufio.Writer
// }

// func New(basePath string, segmentSize time.Duration) *FileStorage {
//   if err := os.MkdirAll(basePath, 0755); err != nil {
//     log.Printf("Failed to create base directory: %v", err)
//   }
//   return &FileStorage{
//     basePath:    basePath,
//     segmentSize: segmentSize,
//   }
// }

// func (f *FileStorage) SaveMessage(roomID string, msg *domain.BaseMessage) error {
//   buffer := f.getOrCreateBuffer(roomID)
//   buffer.mu.Lock()
//   defer buffer.mu.Unlock()

//   buffer.messages = append(buffer.messages, msg)
//   messageSize := int64(len(msg.Payload) + 100) // + метаданные

//   // 🔥 УМНАЯ СТРАТЕГИЯ FLUSH
//   shouldFlush := false
//   switch msg.Type {
//   case domain.TypeSystem, domain.TypeCommand:
//     shouldFlush = true // немедленная запись

//   case domain.TypeText, domain.TypeFile:
//     buffer.size += messageSize
//     shouldFlush = buffer.size >= 64*1024 || time.Since(buffer.lastFlush) > 100*time.Millisecond

//   case domain.TypeTyping, domain.TypeReadReceipt:
//     shouldFlush = time.Since(buffer.lastFlush) > 500*time.Millisecond

//   default:
//     shouldFlush = len(buffer.messages) >= 50
//   }

//   if shouldFlush {
//     return f.flushBuffer(roomID, buffer)
//   }
//   return nil
// }

// func (f *FileStorage) getOrCreateBuffer(roomID string) *RoomBuffer {
//   actual, _ := f.buffers.LoadOrStore(roomID, &RoomBuffer{
//     messages:  make([]*domain.BaseMessage, 0),
//     lastFlush: time.Now(),
//   })
//   return actual.(*RoomBuffer)
// }

// func (f *FileStorage) flushBuffer(roomID string, buffer *RoomBuffer) error {
//   if len(buffer.messages) == 0 {
//     return nil
//   }

//   if err := f.ensureFile(roomID, buffer); err != nil {
//     return err
//   }

//   // 📝 Запись всех сообщений
//   for _, msg := range buffer.messages {
//     line := f.formatMessageLine(msg)
//     if _, err := buffer.writer.WriteString(line); err != nil {
//       return fmt.Errorf("failed to write message: %w", err)
//     }
//   }

//   if err := buffer.writer.Flush(); err != nil {
//     return fmt.Errorf("failed to flush buffer: %w", err)
//   }

//   if err := buffer.activeFile.Sync(); err != nil {
//     log.Printf("Failed to sync file: %v", err)
//   }

//   // 🧹 Очистка буфера
//   buffer.messages = buffer.messages[:0]
//   buffer.size = 0
//   buffer.lastFlush = time.Now()

//   return nil
// }

// func (f *FileStorage) ensureFile(roomID string, buffer *RoomBuffer) error {
//   now := time.Now()
//   filePath := f.getSegmentPath(roomID, now)

//   if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
//     return fmt.Errorf("failed to create directory: %w", err)
//   }

//   file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
//   if err != nil {
//     return fmt.Errorf("failed to open file: %w", err)
//   }

//   buffer.activeFile = file
//   buffer.writer = bufio.NewWriterSize(file, 64*1024) // 64KB буфер
//   return nil
// }

// // 🗂️ УМНАЯ СЕГМЕНТАЦИЯ ПО ТИПАМ КОМНАТ
// func (f *FileStorage) getSegmentPath(roomID string, t time.Time) string {
//   roomType := f.getRoomType(roomID)

//   switch roomType {
//   case domain.RoomTypeVideoCall:
//     // Видеозвонки: 1ч сегменты
//     return filepath.Join(f.basePath, "video_calls", roomID,
//       t.Format("2006-01-02"), fmt.Sprintf("%d.log", t.Hour()))

//   case domain.RoomTypeGroup:
//     // Групповые: 4ч сегменты
//     hour := (t.Hour() / 4) * 4
//     return filepath.Join(f.basePath, "groups", roomID,
//       t.Format("2006-01"), fmt.Sprintf("%s_%02d.log", t.Format("2006-01-02"), hour))

//   case domain.RoomTypePrivate:
//     // Личные: 1 день + шардинг
//     shard := roomID[:2]
//     return filepath.Join(f.basePath, "private", shard, roomID,
//       t.Format("2006-01-02")+".log")

//   case domain.RoomTypePersonal:
//     // Заметки: 1 месяц + шардинг
//     shard := roomID[:2]
//     return filepath.Join(f.basePath, "personal", shard, roomID,
//       t.Format("2006-01")+".log")

//   default:
//     return filepath.Join(f.basePath, "default", roomID,
//       t.Format("2006-01-02")+".log")
//   }
// }

// func (f *FileStorage) formatMessageLine(msg *domain.BaseMessage) string {
//   return fmt.Sprintf("%d|%s|%s|%s|%s|%s\n",
//     msg.Timestamp, msg.ID, msg.Type, msg.UserID, msg.RoomID, string(msg.Payload))
// }
