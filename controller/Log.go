package controller

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

// Logger thread-safe con salida a consola y archivo
type Logger struct {
	level   Level
	file    *os.File
	mu      sync.Mutex
	startAt time.Time
}

// NewLogger crea un logger con nivel mínimo y archivo opcional
func NewLogger(levelStr, logDir string) *Logger {
	level := parseLevel(levelStr)

	// Crear directorio de logs
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("⚠️  No se pudo crear %s: %v\n", logDir, err)
	}

	// Abrir archivo de log del día
	logPath := filepath.Join(logDir, fmt.Sprintf("stream_%s.log",
		time.Now().Format("2006-01-02")))

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("⚠️  No se pudo abrir log file: %v\n", err)
	}

	return &Logger{
		level:   level,
		file:    f,
		startAt: time.Now(),
	}
}

// Close cierra el archivo de log
func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}

func (l *Logger) log(level Level, prefix, msg string, args ...any) {
	if level < l.level {
		return
	}

	formatted := msg
	if len(args) > 0 {
		formatted = fmt.Sprintf(msg, args...)
	}

	ts := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] %s %s\n", ts, prefix, formatted)

	l.mu.Lock()
	defer l.mu.Unlock()

	io.WriteString(os.Stdout, line)
	if l.file != nil {
		l.file.WriteString(line)
	}
}

func (l *Logger) Debug(msg string, args ...any) { l.log(DEBUG, "🔍 DEBUG", msg, args...) }
func (l *Logger) Info(msg string, args ...any)  { l.log(INFO, "ℹ️  INFO ", msg, args...) }
func (l *Logger) Warn(msg string, args ...any)  { l.log(WARN, "⚠️  WARN ", msg, args...) }
func (l *Logger) Error(msg string, args ...any) { l.log(ERROR, "❌ ERROR", msg, args...) }

// InicioProceso mantiene compatibilidad con tu estilo original
func (l *Logger) InicioProceso(name string) {
	sep := strings.Repeat("=", 80)
	l.Info(sep)
	l.Info("🚀 INICIO: %s", name)
	l.Info(sep)
}

func (l *Logger) FinProceso(name string) {
	sep := strings.Repeat("=", 80)
	l.Info(sep)
	l.Info("🛑 FIN: %s (duración: %s)", name, time.Since(l.startAt).Round(time.Millisecond))
	l.Info(sep)
}

func parseLevel(s string) Level {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return DEBUG
	case "WARN", "WARNING":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}
