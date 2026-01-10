package logging

import (
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	
	"github.com/sirupsen/logrus"
)

type writeHook struct {
	Writer []io.Writer
	LogLevels []logrus.Level
}

func (hook *writeHook) Fire(entry *logrus.Entry) error {
	line, err := entry.String()
	if err != nil {
		return err
	}
	for _, w := range hook.Writer {
		w.Write([]byte(line))
	}
	return err
}

func (hook *writeHook) Levels() []logrus.Level {
	return hook.LogLevels
}

var e *logrus.Entry

type Logger struct {
	*logrus.Entry
}

func GetLogger() *Logger {
	return &Logger{e}
}

func (l *Logger) GetLoggerWithField(k string, v interface{}) *Logger {
	return &Logger{l.WithField(k, v)}
}

func Init() {
	l := logrus.New()
	l.SetReportCaller(true)
	
	// Изменяем только формат для Docker
	if os.Getenv("DOCKER_ENV") == "true" || os.Getenv("APP_ENV") == "prod" {
		// JSON формат для Docker/production
		l.Formatter = &logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
				filename := path.Base(frame.File)
				return frame.Function, fmt.Sprintf("%s:%d", filename, frame.Line)
			},
		}
	} else {
		// Текстовый формат для локальной разработки
		l.Formatter = &logrus.TextFormatter{
			CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
				filename := path.Base(frame.File)
				return fmt.Sprintf("%s()", frame.Function), fmt.Sprintf("%s:%d", filename, frame.Line)
			},
			DisableColors: false,
			FullTimestamp: true,
		}
	}
	
	// Пытаемся создать директорию logs, но не паникуем если не получается
	if err := os.MkdirAll("logs", 0755); err != nil {
		// Если не можем создать директорию, просто пишем в stdout
		l.SetOutput(os.Stdout)
		l.Warnf("Cannot create logs directory: %v. Logging to stdout only.", err)
	} else {
		// Открываем файл для логов
		allFile, err := os.OpenFile("logs/all.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
		if err != nil {
			// Если не можем открыть файл, пишем только в stdout
			l.SetOutput(os.Stdout)
			l.Warnf("Cannot open log file: %v. Logging to stdout only.", err)
		} else {
			// Устанавливаем хук для записи в файл и stdout
			l.SetOutput(io.Discard)
			l.AddHook(&writeHook{
				Writer: []io.Writer{allFile, os.Stdout},
				LogLevels: logrus.AllLevels,		
			})
		}
	}
	
	// Устанавливаем уровень логирования из переменной окружения
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	l.SetLevel(level)
	
	e = logrus.NewEntry(l)
}