package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config agrupa toda la configuración del servidor
type Config struct {
	ServerHost          string
	ServerPort          int
	MusicDir            string
	ScanWorkers         int
	SupportedExtensions []string
	LogLevel            string
	LogDir              string
}

// NewConfig carga la configuración desde .env con valores por defecto
func NewConfig() *Config {
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			fmt.Printf("⚠️  No se pudo cargar .env: %v\n", err)
		}
	}

	cfg := &Config{
		ServerHost:          getEnv("SERVER_HOST", "0.0.0.0"),
		ServerPort:          getEnvInt("SERVER_PORT", 8080),
		MusicDir:            getEnv("MUSIC_DIR", "./music"),
		ScanWorkers:         getEnvInt("SCAN_WORKERS", 4),
		SupportedExtensions: parseExtensions(getEnv("SUPPORTED_EXTENSIONS", ".mp3,.flac,.ogg,.m4a,.wav")),
		LogLevel:            getEnv("LOG_LEVEL", "INFO"),
		LogDir:              getEnv("LOG_DIR", "./logs"),
	}

	// Validar directorio de música
	if info, err := os.Stat(cfg.MusicDir); err != nil || !info.IsDir() {
		fmt.Printf("❌ MUSIC_DIR inválido: %s\n", cfg.MusicDir)
		os.Exit(1)
	}

	// Asegurar que el path sea absoluto
	if !filepath.IsAbs(cfg.MusicDir) {
		if abs, err := filepath.Abs(cfg.MusicDir); err == nil {
			cfg.MusicDir = abs
		}
	}

	return cfg
}

// IsSupported verifica si una extensión está soportada
func (c *Config) IsSupported(ext string) bool {
	ext = strings.ToLower(ext)
	for _, e := range c.SupportedExtensions {
		if e == ext {
			return true
		}
	}
	return false
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func parseExtensions(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
