package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// Config configuración del servidor de música
type Config struct {
	Port       string
	MusicPath  string
	AllowedIPs []string
	Log        *Log
}

// NewConfig crea una nueva instancia de Config
func NewConfig() *Config {
	// Cargar .env si existe
	envPath := ".env"
	if _, err := os.Stat(envPath); err == nil {
		if err := godotenv.Load(); err != nil {
			fmt.Printf("⚠️  Advertencia: No se pudo cargar .env: %v\n", err)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	musicPath := os.Getenv("MUSIC_PATH")
	if musicPath == "" {
		musicPath = "./music"
	}

	// Convertir a ruta absoluta
	if absPath, err := filepath.Abs(musicPath); err == nil {
		musicPath = absPath
	}

	// Verificar que exista la carpeta
	if _, err := os.Stat(musicPath); os.IsNotExist(err) {
		fmt.Printf("⚠️  La carpeta de música no existe, creándola: %s\n", musicPath)
		os.MkdirAll(musicPath, 0755)
	}

	allowedIPs := parseAllowedIPs(os.Getenv("ALLOWED_IPS"))

	return &Config{
		Port:       port,
		MusicPath:  musicPath,
		AllowedIPs: allowedIPs,
		Log:        NewLog(),
	}
}

// IsIPAllowed verifica si una IP está en la lista blanca
func (c *Config) IsIPAllowed(ip string) bool {
	if len(c.AllowedIPs) == 0 {
		return true // Sin restricción
	}
	for _, allowed := range c.AllowedIPs {
		if allowed == ip {
			return true
		}
	}
	return false
}

func parseAllowedIPs(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var ips []string
	parts := strings.Split(raw, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			ips = append(ips, p)
		}
	}
	return ips
}

// GetProjectPath retorna la ruta absoluta del proyecto
func (c *Config) GetProjectPath() string {
	path, err := os.Getwd()
	if err != nil {
		return "."
	}
	return path
}
