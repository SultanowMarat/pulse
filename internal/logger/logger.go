// Package logger предоставляет логирование с префиксом сервиса и асинхронной записью,
// чтобы не блокировать основное приложение. Поддерживается логирование времени выполнения функций.
package logger

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

const asyncBufferSize = 8192

var (
	prefix   string
	logLevel = levelInfo
	ch       chan string
	once     sync.Once
)

type level int

const (
	levelDebug level = iota
	levelInfo
)

func initLevel() {
	switch os.Getenv("LOG_LEVEL") {
	case "debug", "trace":
		logLevel = levelDebug
	default:
		logLevel = levelInfo
	}
}

func initWorker() {
	initLevel()
	ch = make(chan string, asyncBufferSize)
	go func() {
		for msg := range ch {
			log.Print(msg)
		}
	}()
}

func enqueue(msg string) {
	once.Do(initWorker)
	select {
	case ch <- msg:
	default:
		// Буфер полон — не блокируем, теряем лог
	}
}

// SetPrefix задаёт префикс для всех последующих логов (например "api", "auth").
func SetPrefix(p string) {
	prefix = p
}

func tag() string {
	if prefix == "" {
		return ""
	}
	return "[" + prefix + "] "
}

// Info пишет в log с префиксом (асинхронно).
func Info(v ...any) {
	enqueue(tag() + fmt.Sprint(v...))
}

// Infof форматирует и пишет с префиксом (асинхронно).
func Infof(format string, v ...any) {
	enqueue(tag() + fmt.Sprintf(format, v...))
}

// Error пишет ошибку с префиксом (асинхронно).
func Error(v ...any) {
	enqueue(tag() + "ERROR: " + fmt.Sprint(v...))
}

// Errorf форматирует ошибку с префиксом (асинхронно).
func Errorf(format string, v ...any) {
	enqueue(tag() + "ERROR: " + fmt.Sprintf(format, v...))
}

// LogDuration логирует имя функции и время выполнения в миллисекундах (асинхронно).
// При LOG_LEVEL=info логирует только вызовы дольше 100ms; при LOG_LEVEL=debug — все.
func LogDuration(fn string, start time.Time) {
	elapsed := time.Since(start)
	if logLevel == levelDebug || elapsed >= 100*time.Millisecond {
		enqueue(fmt.Sprintf("%sfn=%s duration_ms=%d", tag(), fn, elapsed.Milliseconds()))
	}
}

// DeferLogDuration возвращает функцию для вызова в defer: defer logger.DeferLogDuration("HandlerName", time.Now())().
func DeferLogDuration(fn string, start time.Time) func() {
	return func() { LogDuration(fn, start) }
}
