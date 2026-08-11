package controller

import "time"

// Track representa una canción en la biblioteca
type Track struct {
	ID       string        `json:"id"` // Hash SHA1 del path
	Path     string        `json:"-"`  // Ruta absoluta (no expuesta)
	Filename string        `json:"filename"`
	Title    string        `json:"title"`
	Artist   string        `json:"artist"`
	Album    string        `json:"album"`
	Year     int           `json:"year,omitempty"`
	Duration time.Duration `json:"duration_ms"` // en milisegundos
	Format   string        `json:"format"`      // mp3, flac, etc.
	Size     int64         `json:"size_bytes"`
	Modified time.Time     `json:"modified"`
}
