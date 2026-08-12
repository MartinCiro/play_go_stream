package controller

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// PlaybackState representa el estado de la reproducción
type PlaybackState string

const (
	StatePlaying PlaybackState = "playing"
	StatePaused  PlaybackState = "paused"
	StateStopped PlaybackState = "stopped"
)

// PlaybackStatus es el estado completo de la reproducción
type PlaybackStatus struct {
	State       PlaybackState `json:"state"`
	CurrentSong *Song         `json:"current_song,omitempty"`
	Position    float64       `json:"position"`
	Queue       []*Song       `json:"queue"`
	QueueIndex  int           `json:"queue_index"`
	QueueLength int           `json:"queue_length"`
	RepeatMode  string        `json:"repeat_mode"`
	Shuffle     bool          `json:"shuffle"`
}

// PlaybackController gestiona el estado de reproducción del servidor
type PlaybackController struct {
	config       *Config
	musicService *MusicService

	// Cola actual (puede estar en orden original o shuffled)
	queue      []*Song
	queueIndex int
	state      PlaybackState
	position   float64
	startedAt  time.Time
	repeatMode string

	// ──────────────────────────────────────────────
	// CAMPOS NUEVOS PARA SHUFFLE
	// ──────────────────────────────────────────────
	shuffle        bool       // ¿Está activo el shuffle?
	originalQueue  []*Song    // Orden original (preservado siempre)
	originalIndex  int        // Índice original actual (para restaurar)
	shuffleHistory []int      // Índices ya reproducidos en este ciclo shuffle
	shuffleRng     *rand.Rand // RNG con seed para reproducibilidad

	mu sync.RWMutex
}

// NewPlaybackController crea un nuevo controlador de reproducción
func NewPlaybackController(config *Config, musicService *MusicService) *PlaybackController {
	return &PlaybackController{
		config:        config,
		musicService:  musicService,
		queue:         make([]*Song, 0),
		originalQueue: make([]*Song, 0),
		queueIndex:    -1,
		originalIndex: -1,
		state:         StateStopped,
		repeatMode:    "none",
		shuffleRng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Play inicia o reanuda la reproducción
func (c *PlaybackController) Play() (*PlaybackStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.queue) == 0 {
		return nil, fmt.Errorf("la cola está vacía, agrega canciones primero")
	}

	if c.queueIndex < 0 || c.queueIndex >= len(c.queue) {
		c.queueIndex = 0
		c.position = 0
	}

	c.state = StatePlaying
	c.startedAt = time.Now()

	if c.config.Log != nil {
		c.config.Log.Comentario("INFO", fmt.Sprintf("▶ Play: %s (pos: %.1fs)",
			c.queue[c.queueIndex].Title, c.position))
	}

	return c.getStatusLocked(), nil
}

// Pause pausa la reproducción y guarda la posición actual
func (c *PlaybackController) Pause() (*PlaybackStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != StatePlaying {
		return nil, fmt.Errorf("no hay reproducción en curso")
	}

	c.position = c.currentPositionLocked()
	c.state = StatePaused

	if c.config.Log != nil {
		c.config.Log.Comentario("INFO", fmt.Sprintf("⏸ Pause: %s en %.1fs",
			c.queue[c.queueIndex].Title, c.position))
	}

	return c.getStatusLocked(), nil
}

// Stop detiene completamente la reproducción
func (c *PlaybackController) Stop() (*PlaybackStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.state = StateStopped
	c.position = 0

	if c.config.Log != nil {
		c.config.Log.Comentario("INFO", "⏹ Stop")
	}

	return c.getStatusLocked(), nil
}

// ──────────────────────────────────────────────
// Next - CON SOPORTE DE SHUFFLE
// ──────────────────────────────────────────────
func (c *PlaybackController) Next() (*PlaybackStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.queue) == 0 {
		return nil, fmt.Errorf("la cola está vacía")
	}

	wasPlaying := c.state == StatePlaying

	// Modo Repeat One: reiniciar canción actual
	if c.repeatMode == "one" {
		c.position = 0
		c.startedAt = time.Now()
		return c.getStatusLocked(), nil
	}

	if c.shuffle {
		// MODO SHUFFLE: elegir siguiente del pool aleatorio
		c.advanceShuffleLocked()
	} else {
		// MODO NORMAL: siguiente secuencial
		nextIndex := c.queueIndex + 1
		if nextIndex >= len(c.queue) {
			if c.repeatMode == "all" {
				nextIndex = 0
			} else {
				// Fin de la cola
				c.state = StateStopped
				c.position = 0
				c.queueIndex = -1
				if c.config.Log != nil {
					c.config.Log.Comentario("INFO", "⏭ Fin de la cola")
				}
				return c.getStatusLocked(), nil
			}
		}
		c.queueIndex = nextIndex
		c.position = 0
	}

	if wasPlaying {
		c.state = StatePlaying
		c.startedAt = time.Now()
	}

	if c.config.Log != nil && c.queueIndex >= 0 {
		c.config.Log.Comentario("INFO", fmt.Sprintf("⏭ Next: %s", c.queue[c.queueIndex].Title))
	}

	return c.getStatusLocked(), nil
}

// advanceShuffleLocked elige la siguiente canción en modo shuffle
func (c *PlaybackController) advanceShuffleLocked() {
	// Guardar canción actual en historial
	if c.queueIndex >= 0 && c.queueIndex < len(c.queue) {
		c.shuffleHistory = append(c.shuffleHistory, c.queueIndex)
	}

	// Construir pool de canciones NO reproducidas aún
	unplayed := make([]int, 0, len(c.queue))
	playedSet := make(map[int]bool)
	for _, idx := range c.shuffleHistory {
		playedSet[idx] = true
	}
	for i := 0; i < len(c.queue); i++ {
		if !playedSet[i] {
			unplayed = append(unplayed, i)
		}
	}

	// Si ya tocamos todas, reiniciar historial (nuevo ciclo)
	if len(unplayed) == 0 {
		if c.config.Log != nil {
			c.config.Log.Comentario("INFO", "🔀 Shuffle: todas las canciones tocadas, reiniciando ciclo")
		}
		// Mantener la última canción en historial, resetear el resto
		if len(c.shuffleHistory) > 0 {
			last := c.shuffleHistory[len(c.shuffleHistory)-1]
			c.shuffleHistory = []int{last}
		}
		// Reconstruir pool excluyendo la última reproducida
		for i := 0; i < len(c.queue); i++ {
			last := 0
			if i != last {
				unplayed = append(unplayed, i)
			}
		}
	}

	// Elegir una canción aleatoria del pool
	pick := unplayed[c.shuffleRng.Intn(len(unplayed))]
	c.queueIndex = pick
	c.position = 0
}

// ──────────────────────────────────────────────
// Previous - CON SOPORTE DE SHUFFLE
// ──────────────────────────────────────────────
func (c *PlaybackController) Previous() (*PlaybackStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.queue) == 0 {
		return nil, fmt.Errorf("la cola está vacía")
	}

	wasPlaying := c.state == StatePlaying

	// Si lleva más de 3 segundos, reiniciar canción actual
	currentPos := c.currentPositionLocked()
	if currentPos > 3.0 {
		c.position = 0
		if wasPlaying {
			c.startedAt = time.Now()
		}
		if c.config.Log != nil {
			c.config.Log.Comentario("INFO", "⏮ Reiniciando canción actual")
		}
		return c.getStatusLocked(), nil
	}

	if c.shuffle {
		// MODO SHUFFLE: retroceder en el historial
		if len(c.shuffleHistory) > 0 {
			prevIdx := c.shuffleHistory[len(c.shuffleHistory)-1]
			c.shuffleHistory = c.shuffleHistory[:len(c.shuffleHistory)-1]
			c.queueIndex = prevIdx
			c.position = 0

			if c.config.Log != nil {
				c.config.Log.Comentario("INFO", fmt.Sprintf("⏮ Shuffle Previous: %s", c.queue[c.queueIndex].Title))
			}
		} else {
			// No hay historial, reiniciar la actual
			c.position = 0
			if c.config.Log != nil {
				c.config.Log.Comentario("INFO", "⏮ No hay historial, reiniciando canción actual")
			}
		}
	} else {
		// MODO NORMAL: anterior secuencial
		prevIndex := c.queueIndex - 1
		if prevIndex < 0 {
			if c.repeatMode == "all" {
				prevIndex = len(c.queue) - 1
			} else {
				prevIndex = 0
			}
		}
		c.queueIndex = prevIndex
		c.position = 0

		if c.config.Log != nil {
			c.config.Log.Comentario("INFO", fmt.Sprintf("⏮ Previous: %s", c.queue[c.queueIndex].Title))
		}
	}

	if wasPlaying {
		c.state = StatePlaying
		c.startedAt = time.Now()
	}

	return c.getStatusLocked(), nil
}

// AddToQueue agrega una canción al final de la cola
func (c *PlaybackController) AddToQueue(songID string) (*PlaybackStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	song, exists := c.musicService.GetByID(songID)
	if !exists {
		return nil, fmt.Errorf("canción no encontrada: %s", songID)
	}

	c.queue = append(c.queue, song)
	c.originalQueue = append(c.originalQueue, song) // ← NUEVO: preservar original

	if c.config.Log != nil {
		c.config.Log.Comentario("INFO", fmt.Sprintf("➕ Agregada a cola: %s", song.Title))
	}

	return c.getStatusLocked(), nil
}

// PlayNow agrega una canción y la reproduce inmediatamente
func (c *PlaybackController) PlayNow(songID string) (*PlaybackStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	song, exists := c.musicService.GetByID(songID)
	if !exists {
		return nil, fmt.Errorf("canción no encontrada: %s", songID)
	}

	insertIndex := c.queueIndex + 1
	if insertIndex < 0 {
		insertIndex = 0
	}

	// Insertar en cola actual
	newQueue := make([]*Song, 0, len(c.queue)+1)
	newQueue = append(newQueue, c.queue[:insertIndex]...)
	newQueue = append(newQueue, song)
	newQueue = append(newQueue, c.queue[insertIndex:]...)
	c.queue = newQueue

	// Insertar también en cola original (mismo índice)
	newOrig := make([]*Song, 0, len(c.originalQueue)+1)
	origInsert := insertIndex
	if origInsert > len(c.originalQueue) {
		origInsert = len(c.originalQueue)
	}
	newOrig = append(newOrig, c.originalQueue[:origInsert]...)
	newOrig = append(newOrig, song)
	newOrig = append(newOrig, c.originalQueue[origInsert:]...)
	c.originalQueue = newOrig

	c.queueIndex = insertIndex
	c.position = 0
	c.state = StatePlaying
	c.startedAt = time.Now()

	if c.config.Log != nil {
		c.config.Log.Comentario("INFO", fmt.Sprintf("🎵 Play Now: %s", song.Title))
	}

	return c.getStatusLocked(), nil
}

// SetQueue reemplaza toda la cola con una lista de IDs
func (c *PlaybackController) SetQueue(songIDs []string) (*PlaybackStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	newQueue := make([]*Song, 0, len(songIDs))
	for _, id := range songIDs {
		if song, exists := c.musicService.GetByID(id); exists {
			newQueue = append(newQueue, song)
		}
	}

	c.queue = newQueue
	c.originalQueue = make([]*Song, len(newQueue))
	copy(c.originalQueue, newQueue) // Preservar orden original

	c.queueIndex = -1
	c.state = StateStopped
	c.position = 0

	// Limpiar estado de shuffle
	c.shuffleHistory = nil

	if c.config.Log != nil {
		c.config.Log.Comentario("INFO", fmt.Sprintf("📋 Cola reemplazada: %d canciones", len(c.queue)))
	}

	return c.getStatusLocked(), nil
}

// ClearQueue vacía la cola
func (c *PlaybackController) ClearQueue() (*PlaybackStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.queue = make([]*Song, 0)
	c.originalQueue = make([]*Song, 0)
	c.queueIndex = -1
	c.state = StateStopped
	c.position = 0
	c.shuffleHistory = nil

	if c.config.Log != nil {
		c.config.Log.Comentario("INFO", "🗑 Cola limpiada")
	}

	return c.getStatusLocked(), nil
}

// RemoveFromQueue elimina una canción de la cola por índice
func (c *PlaybackController) RemoveFromQueue(index int) (*PlaybackStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.queue) {
		return nil, fmt.Errorf("índice fuera de rango: %d", index)
	}

	// Identificar la canción a eliminar para buscarla también en originalQueue
	songToRemove := c.queue[index]

	// Eliminar de cola actual
	c.queue = append(c.queue[:index], c.queue[index+1:]...)

	// Eliminar de cola original
	for i, s := range c.originalQueue {
		if s.ID == songToRemove.ID {
			c.originalQueue = append(c.originalQueue[:i], c.originalQueue[i+1:]...)
			break
		}
	}

	// Ajustar índices
	if c.queueIndex == index {
		if len(c.queue) == 0 {
			c.queueIndex = -1
			c.state = StateStopped
			c.position = 0
		} else if c.queueIndex >= len(c.queue) {
			c.queueIndex = 0
		}
	} else if c.queueIndex > index {
		c.queueIndex--
	}

	// Limpiar historial de shuffle (ya no es válido)
	c.shuffleHistory = nil

	if c.config.Log != nil {
		c.config.Log.Comentario("INFO", fmt.Sprintf("➖ Eliminada de cola: %s", songToRemove.Title))
	}

	return c.getStatusLocked(), nil
}

// Seek cambia la posición de reproducción
func (c *PlaybackController) Seek(position float64) (*PlaybackStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.queueIndex < 0 || c.queueIndex >= len(c.queue) {
		return nil, fmt.Errorf("no hay canción en reproducción")
	}

	if position < 0 {
		position = 0
	}

	c.position = position
	c.startedAt = time.Now()

	if c.config.Log != nil {
		c.config.Log.Comentario("INFO", fmt.Sprintf("⏩ Seek a %.1fs", position))
	}

	return c.getStatusLocked(), nil
}

// SetRepeatMode establece el modo de repetición
func (c *PlaybackController) SetRepeatMode(mode string) (*PlaybackStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	validModes := map[string]bool{"none": true, "one": true, "all": true}
	if !validModes[mode] {
		return nil, fmt.Errorf("modo inválido: %s (válido: none, one, all)", mode)
	}

	c.repeatMode = mode

	if c.config.Log != nil {
		c.config.Log.Comentario("INFO", fmt.Sprintf("🔁 Repeat mode: %s", mode))
	}

	return c.getStatusLocked(), nil
}

// ToggleRepeat alterna entre los modos de repetición
func (c *PlaybackController) ToggleRepeat() (*PlaybackStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.repeatMode {
	case "none":
		c.repeatMode = "all"
	case "all":
		c.repeatMode = "one"
	case "one":
		c.repeatMode = "none"
	}

	if c.config.Log != nil {
		c.config.Log.Comentario("INFO", fmt.Sprintf("🔁 Repeat mode: %s", c.repeatMode))
	}

	return c.getStatusLocked(), nil
}

// ──────────────────────────────────────────────
// ToggleShuffle - NUEVO MÉTODO
// ──────────────────────────────────────────────
func (c *PlaybackController) ToggleShuffle() (*PlaybackStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.queue) < 2 {
		return nil, fmt.Errorf("se necesitan al menos 2 canciones para activar shuffle")
	}

	c.shuffle = !c.shuffle

	if c.shuffle {
		c.activateShuffleLocked()
	} else {
		c.deactivateShuffleLocked()
	}

	return c.getStatusLocked(), nil
}

// SetShuffle establece explícitamente el estado del shuffle
func (c *PlaybackController) SetShuffle(enabled bool) (*PlaybackStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.shuffle == enabled {
		return c.getStatusLocked(), nil // Ya está en el estado deseado
	}

	if len(c.queue) < 2 && enabled {
		return nil, fmt.Errorf("se necesitan al menos 2 canciones para activar shuffle")
	}

	c.shuffle = enabled

	if enabled {
		c.activateShuffleLocked()
	} else {
		c.deactivateShuffleLocked()
	}

	return c.getStatusLocked(), nil
}

// activateShuffleLocked activa el modo shuffle
func (c *PlaybackController) activateShuffleLocked() {
	// Guardar el índice original de la canción actual
	if c.queueIndex >= 0 {
		c.originalIndex = c.queueIndex
	} else {
		c.originalIndex = 0
	}

	// Preservar la canción actual en su lugar
	currentSong := c.queue[c.queueIndex]

	// Crear pool de canciones "siguientes" (todas excepto la actual)
	remaining := make([]*Song, 0, len(c.queue)-1)
	for i, s := range c.queue {
		if i != c.queueIndex {
			remaining = append(remaining, s)
		}
	}

	// Fisher-Yates shuffle del pool restante
	c.fisherYatesShuffle(remaining)

	// Reconstruir cola: [canción actual] + [restante barajado]
	c.queue = make([]*Song, 0, len(c.queue))
	c.queue = append(c.queue, currentSong)
	c.queue = append(c.queue, remaining...)

	c.queueIndex = 0 // La canción actual ahora está en el índice 0

	// Inicializar historial de shuffle (empezamos con la actual)
	c.shuffleHistory = []int{0}

	if c.config.Log != nil {
		c.config.Log.Comentario("INFO", "🔀 Shuffle ACTIVADO")
	}
}

// deactivateShuffleLocked desactiva el modo shuffle y restaura el orden original
func (c *PlaybackController) deactivateShuffleLocked() {
	// Encontrar la canción actual en el orden original
	var currentSong *Song
	if c.queueIndex >= 0 && c.queueIndex < len(c.queue) {
		currentSong = c.queue[c.queueIndex]
	}

	// Restaurar cola original
	c.queue = make([]*Song, len(c.originalQueue))
	copy(c.queue, c.originalQueue)

	// Encontrar el índice de la canción actual en el orden original
	c.queueIndex = -1
	if currentSong != nil {
		for i, s := range c.queue {
			if s.ID == currentSong.ID {
				c.queueIndex = i
				break
			}
		}
	}

	if c.queueIndex < 0 {
		c.queueIndex = 0
	}

	// Limpiar historial
	c.shuffleHistory = nil

	if c.config.Log != nil {
		c.config.Log.Comentario("INFO", "🔀 Shuffle DESACTIVADO - orden original restaurado")
	}
}

// fisherYatesShuffle implementa el algoritmo Fisher-Yates (Knuth shuffle)
// Es un shuffle imparcial: cada permutación tiene la misma probabilidad
func (c *PlaybackController) fisherYatesShuffle(songs []*Song) {
	n := len(songs)
	for i := n - 1; i > 0; i-- {
		j := c.shuffleRng.Intn(i + 1)
		songs[i], songs[j] = songs[j], songs[i]
	}
}

// GetStatus retorna el estado actual (thread-safe)
func (c *PlaybackController) GetStatus() *PlaybackStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.getStatusLocked()
}

// getStatusLocked retorna el estado (debe llamarse con el mutex tomado)
func (c *PlaybackController) getStatusLocked() *PlaybackStatus {
	status := &PlaybackStatus{
		State:       c.state,
		Queue:       c.queue,
		QueueIndex:  c.queueIndex,
		QueueLength: len(c.queue),
		RepeatMode:  c.repeatMode,
		Shuffle:     c.shuffle,
		Position:    c.currentPositionLocked(),
	}

	if c.queueIndex >= 0 && c.queueIndex < len(c.queue) {
		status.CurrentSong = c.queue[c.queueIndex]
	}

	return status
}

// currentPositionLocked calcula la posición actual en segundos
func (c *PlaybackController) currentPositionLocked() float64 {
	if c.state == StatePlaying {
		elapsed := time.Since(c.startedAt).Seconds()
		return c.position + elapsed
	}
	return c.position
}
