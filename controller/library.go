package controller

import (
	"crypto/sha1"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dhowden/tag"
)

// Library es la colección indexada de tracks
type Library struct {
	cfg    *Config
	log    *Logger
	tracks map[string]*Track // ID -> Track
	mu     sync.RWMutex
}

// NewLibrary crea una biblioteca vacía
func NewLibrary(cfg *Config, log *Logger) *Library {
	return &Library{
		cfg:    cfg,
		log:    log,
		tracks: make(map[string]*Track),
	}
}

// Scan recorre MUSIC_DIR y construye el índice
func (l *Library) Scan() error {
	start := time.Now()
	l.log.Info("Iniciando escaneo de %s con %d workers", l.cfg.MusicDir, l.cfg.ScanWorkers)

	// Canal de paths a procesar
	paths := make(chan string, 1024)
	results := make(chan *Track, 1024)

	// Workers
	var wg sync.WaitGroup
	for i := 0; i < l.cfg.ScanWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range paths {
				if t := l.processFile(p); t != nil {
					results <- t
				}
			}
		}()
	}

	// Recolector
	done := make(chan struct{})
	go func() {
		for t := range results {
			l.mu.Lock()
			l.tracks[t.ID] = t
			l.mu.Unlock()
		}
		close(done)
	}()

	// Walk del FS
	err := filepath.WalkDir(l.cfg.MusicDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			l.log.Warn("Error accediendo %s: %v", path, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if l.cfg.IsSupported(filepath.Ext(path)) {
			paths <- path
		}
		return nil
	})

	close(paths)
	wg.Wait()
	close(results)
	<-done

	l.log.Info("✅ Escaneo completado: %d tracks en %s",
		len(l.tracks), time.Since(start).Round(time.Millisecond))

	if err != nil {
		return fmt.Errorf("walk dir: %w", err)
	}
	return nil
}

// Get retorna un track por ID
func (l *Library) Get(id string) (*Track, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	t, ok := l.tracks[id]
	return t, ok
}

// All retorna todos los tracks (copia)
func (l *Library) All() []*Track {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]*Track, 0, len(l.tracks))
	for _, t := range l.tracks {
		out = append(out, t)
	}
	return out
}

// Count retorna el número total de tracks
func (l *Library) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.tracks)
}

// processFile lee metadata de un archivo y construye un Track
func (l *Library) processFile(path string) *Track {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	t := &Track{
		ID:       hashPath(path),
		Path:     path,
		Filename: filepath.Base(path),
		Format:   strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."),
		Size:     info.Size(),
		Modified: info.ModTime(),
		Title:    info.Name(), // fallback
	}

	// Intentar leer tags (puede fallar en archivos corruptos)
	if f, err := os.Open(path); err == nil {
		if m, err := tag.ReadFrom(f); err == nil {
			if v := m.Title(); v != "" {
				t.Title = v
			}
			if v := m.Artist(); v != "" {
				t.Artist = v
			}
			if v := m.Album(); v != "" {
				t.Album = v
			}
			t.Year = m.Year()
		}
		f.Close()
	}

	return t
}

func hashPath(path string) string {
	h := sha1.Sum([]byte(path))
	return fmt.Sprintf("%x", h[:8]) // 16 chars son suficientes
}
