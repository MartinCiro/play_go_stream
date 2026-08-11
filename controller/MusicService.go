package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

// MusicState representa el estado actual del reproductor
type MusicState struct {
	Playing  bool          `json:"playing"`
	Paused   bool          `json:"paused"`
	Current  *Track        `json:"current"`
	Volume   float64       `json:"volume"`
	Position time.Duration `json:"position"`
	Duration time.Duration `json:"duration"`
}

// MusicService gestiona la reproducción de audio
type MusicService struct {
	mu           sync.RWMutex
	log          *Logger
	libraryPath  string
	tracks       []*Track
	streamer     beep.StreamSeekCloser
	ctrl         *beep.Ctrl
	volume       *beep.Volume
	currentTrack *Track
	playing      bool
	paused       bool
	done         chan struct{}
	sampleRate   beep.SampleRate
	initialized  bool
}

// NewMusicService crea una nueva instancia del servicio
func NewMusicService(config *Config) *MusicService {
	return &MusicService{
		log:         config.LogDir,
		libraryPath: config.MusicLibraryPath,
		tracks:      make([]*Track, 0),
		volume: &beep.Volume{
			Base:   1,
			Silent: false,
		},
		done:       make(chan struct{}),
		sampleRate: 44100,
	}
}

// Initialize prepara el speaker y carga la librería
func (m *MusicService) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return nil
	}

	// Inicializar speaker (44100 Hz es estándar)
	if err := speaker.Init(m.sampleRate, m.sampleRate.N(time.Second/10)); err != nil {
		if m.log != nil {
			m.log.Error(fmt.Sprintf("No se pudo inicializar el speaker: %v", err), "MusicService")
		}
		return fmt.Errorf("error inicializando speaker: %v", err)
	}

	// Cargar librería
	if err := m.loadLibrary(); err != nil {
		return err
	}

	m.initialized = true

	if m.log != nil {
		m.log.Comentario("SUCCESS", fmt.Sprintf("MusicService inicializado: %d canciones cargadas", len(m.tracks)))
	}

	return nil
}

// loadLibrary escanea la carpeta de música y carga los archivos
func (m *MusicService) loadLibrary() error {
	if m.libraryPath == "" {
		return fmt.Errorf("ruta de librería no configurada (MUSIC_LIBRARY_PATH)")
	}

	if _, err := os.Stat(m.libraryPath); os.IsNotExist(err) {
		return fmt.Errorf("la carpeta de música no existe: %s", m.libraryPath)
	}

	m.tracks = m.tracks[:0] // Limpiar lista

	extensions := map[string]bool{
		".mp3":  true,
		".flac": true,
		".wav":  true,
		".ogg":  true,
	}

	err := filepath.Walk(m.libraryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(info.Name()))
		if !extensions[ext] {
			return nil
		}

		track, err := m.scanTrack(path)
		if err != nil {
			if m.log != nil {
				m.log.Comentario("WARNING", fmt.Sprintf("No se pudo escanear %s: %v", info.Name(), err))
			}
			return nil
		}

		m.tracks = append(m.tracks, track)
		return nil
	})

	if err != nil {
		return fmt.Errorf("error escaneando librería: %v", err)
	}

	// Ordenar por título
	sort.Slice(m.tracks, func(i, j int) bool {
		return m.tracks[i].Title < m.tracks[j].Title
	})

	return nil
}

// scanTrack analiza un archivo y extrae metadatos básicos
func (m *MusicService) scanTrack(path string) (*Track, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var streamer beep.StreamSeekCloser
	var format beep.Format

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		streamer, format, err = mp3.Decode(file)
	case ".flac":
		streamer, format, err = flac.Decode(file)
	case ".wav":
		streamer, format, err = wav.Decode(file)
	case ".ogg":
		streamer, format, err = vorbis.Decode(file)
	default:
		return nil, fmt.Errorf("formato no soportado: %s", ext)
	}

	if err != nil {
		return nil, err
	}
	defer streamer.Close()

	// Generar ID basado en el nombre del archivo
	baseName := strings.TrimSuffix(filepath.Base(path), ext)
	id := fmt.Sprintf("%x", time.Now().UnixNano())[:8]

	// Título por defecto = nombre del archivo sin extensión
	title := baseName
	artist := "Desconocido"

	return &Track{
		ID:       id,
		Title:    title,
		Artist:   artist,
		Filename: path,
		Duration: format.SampleRate.D(streamer.Len()),
	}, nil
}

// ListTracks retorna la lista de canciones disponibles
func (m *MusicService) ListTracks() []*Track {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Copia defensiva
	result := make([]*Track, len(m.tracks))
	copy(result, m.tracks)
	return result
}

// Play reproduce una canción por ID
func (m *MusicService) Play(trackID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Buscar track
	var target *Track
	for _, t := range m.tracks {
		if t.ID == trackID {
			target = t
			break
		}
	}

	if target == nil {
		return fmt.Errorf("canción no encontrada: %s", trackID)
	}

	// Detener reproducción actual si existe
	if m.playing {
		m.stopLocked()
	}

	// Abrir archivo
	file, err := os.Open(target.Filename)
	if err != nil {
		return fmt.Errorf("error abriendo archivo: %v", err)
	}

	var streamer beep.StreamSeekCloser
	var format beep.Format

	ext := strings.ToLower(filepath.Ext(target.Filename))
	switch ext {
	case ".mp3":
		streamer, format, err = mp3.Decode(file)
	case ".flac":
		streamer, format, err = flac.Decode(file)
	case ".wav":
		streamer, format, err = wav.Decode(file)
	case ".ogg":
		streamer, format, err = vorbis.Decode(file)
	}

	if err != nil {
		file.Close()
		return fmt.Errorf("error decodificando: %v", err)
	}

	// Resample si es necesario (para uniformar)
	resampled := beep.Resample(4, format.SampleRate, m.sampleRate, streamer)

	// Construir cadena: streamer -> volume -> ctrl
	m.streamer = streamer
	m.volume.Streamer = resampled
	m.ctrl = &beep.Ctrl{Streamer: m.volume}

	done := make(chan bool)
	speaker.Play(beep.Seq(m.ctrl, beep.Callback(func() {
		done <- true
	})))

	m.currentTrack = target
	m.playing = true
	m.paused = false

	if m.log != nil {
		m.log.Comentario("INFO", fmt.Sprintf("Reproduciendo: %s - %s", target.Artist, target.Title))
	}

	// Goroutine para detectar fin de canción
	go func() {
		<-done
		m.mu.Lock()
		m.playing = false
		m.paused = false
		m.streamer.Close()
		file.Close()
		m.mu.Unlock()

		if m.log != nil {
			m.log.Comentario("INFO", "Canción finalizada")
		}
	}()

	return nil
}

// Pause pausa la reproducción
func (m *MusicService) Pause() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.playing || m.paused {
		return fmt.Errorf("no hay música reproduciéndose")
	}

	speaker.Lock()
	m.ctrl.Paused = true
	speaker.Unlock()

	m.paused = true

	if m.log != nil {
		m.log.Comentario("INFO", "Reproducción pausada")
	}

	return nil
}

// Resume continúa la reproducción
func (m *MusicService) Resume() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.playing || !m.paused {
		return fmt.Errorf("no hay música pausada")
	}

	speaker.Lock()
	m.ctrl.Paused = false
	speaker.Unlock()

	m.paused = false

	if m.log != nil {
		m.log.Comentario("INFO", "Reproducción reanudada")
	}

	return nil
}

// Stop detiene la reproducción
func (m *MusicService) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.playing {
		return fmt.Errorf("no hay música reproduciéndose")
	}

	m.stopLocked()

	if m.log != nil {
		m.log.Comentario("INFO", "Reproducción detenida")
	}

	return nil
}

// stopLocked detiene la reproducción (asume que el mutex está tomado)
func (m *MusicService) stopLocked() {
	speaker.Clear()

	if m.streamer != nil {
		m.streamer.Close()
		m.streamer = nil
	}

	m.playing = false
	m.paused = false
	m.currentTrack = nil
}

// SetVolume ajusta el volumen (0.0 = silencio, 1.0 = máximo)
func (m *MusicService) SetVolume(vol float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if vol < 0 {
		vol = 0
	}
	if vol > 1 {
		vol = 1
	}

	speaker.Lock()
	if m.volume != nil {
		m.volume.Base = vol
	}
	speaker.Unlock()

	if m.log != nil {
		m.log.Comentario("INFO", fmt.Sprintf("Volumen ajustado a %.0f%%", vol*100))
	}

	return nil
}

// GetState retorna el estado actual del reproductor
func (m *MusicService) GetState() MusicState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var position time.Duration
	var duration time.Duration

	if m.currentTrack != nil && m.streamer != nil {
		format := beep.Format{SampleRate: m.sampleRate, NumChannels: 2, Precision: 2}
		position = m.streamer.Position().D(format)
		duration = m.currentTrack.Duration
	}

	return MusicState{
		Playing:  m.playing,
		Paused:   m.paused,
		Current:  m.currentTrack,
		Volume:   m.volume.Base,
		Position: position,
		Duration: duration,
	}
}

// Shutdown limpia recursos
func (m *MusicService) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.playing {
		m.stopLocked()
	}

	speaker.Clear()
	speaker.Close()

	if m.log != nil {
		m.log.Comentario("INFO", "MusicService apagado")
	}
}
