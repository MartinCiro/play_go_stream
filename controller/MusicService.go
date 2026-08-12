package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Song representa una canción en la biblioteca
type Song struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	Filename string  `json:"filename"`
	Path     string  `json:"path"`
	Size     int64   `json:"size"`
	Duration float64 `json:"duration"` // en segundos (si se lee metadata)
}

// MusicService gestiona la biblioteca de música
type MusicService struct {
	config *Config
	songs  map[string]*Song
	mu     sync.RWMutex
}

// NewMusicService crea un nuevo servicio de música
func NewMusicService(config *Config) *MusicService {
	return &MusicService{
		config: config,
		songs:  make(map[string]*Song),
	}
}

// LoadLibrary escanea la carpeta de música y carga todas las canciones
func (s *MusicService) LoadLibrary() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Limpiar biblioteca anterior
	s.songs = make(map[string]*Song)

	if s.config.Log != nil {
		s.config.Log.Comentario("INFO", fmt.Sprintf("Escaneando biblioteca en: %s", s.config.MusicPath))
	}

	// Extensiones de audio soportadas
	audioExts := map[string]bool{
		".mp3":  true,
		".flac": true,
		".wav":  true,
		".ogg":  true,
		".m4a":  true,
		".aac":  true,
		".opus": true,
	}

	count := 0

	err := filepath.Walk(s.config.MusicPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continuar si hay error en un archivo
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !audioExts[ext] {
			return nil
		}

		// Generar ID basado en ruta relativa
		relPath, _ := filepath.Rel(s.config.MusicPath, path)
		id := strings.ReplaceAll(relPath, string(os.PathSeparator), "_")
		id = strings.ReplaceAll(id, " ", "_")

		// Extraer título del nombre del archivo (sin extensión)
		title := strings.TrimSuffix(info.Name(), ext)
		artist := "Desconocido"

		// Intentar extraer "Artista - Título" del nombre
		if parts := strings.SplitN(title, " - ", 2); len(parts) == 2 {
			artist = parts[0]
			title = parts[1]
		}

		song := &Song{
			ID:       id,
			Title:    title,
			Artist:   artist,
			Filename: info.Name(),
			Path:     path,
			Size:     info.Size(),
			Duration: 0, // TODO: leer metadata con librería como go-taglib
		}

		s.songs[id] = song
		count++

		return nil
	})

	if err != nil {
		return fmt.Errorf("error escaneando biblioteca: %v", err)
	}

	if s.config.Log != nil {
		s.config.Log.Comentario("SUCCESS", fmt.Sprintf("Biblioteca cargada: %d canciones", count))
	}

	return nil
}

// GetAll retorna todas las canciones
func (s *MusicService) GetAll() []*Song {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Song, 0, len(s.songs))
	for _, song := range s.songs {
		result = append(result, song)
	}
	return result
}

// GetByID retorna una canción por su ID
func (s *MusicService) GetByID(id string) (*Song, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	song, exists := s.songs[id]
	return song, exists
}

// Search busca canciones por título o artista
func (s *MusicService) Search(query string) []*Song {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query = strings.ToLower(query)
	var results []*Song

	for _, song := range s.songs {
		if strings.Contains(strings.ToLower(song.Title), query) ||
			strings.Contains(strings.ToLower(song.Artist), query) {
			results = append(results, song)
		}
	}

	return results
}

// Count retorna el número de canciones
func (s *MusicService) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.songs)
}

// GetFilePath retorna la ruta física de una canción
func (s *MusicService) GetFilePath(id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	song, exists := s.songs[id]
	if !exists {
		return "", fmt.Errorf("canción no encontrada: %s", id)
	}

	return song.Path, nil
}
