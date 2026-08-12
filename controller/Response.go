package controller

import (
	"encoding/json"
	"net/http"
)

// APIResponse respuesta estándar de la API
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// JSON envía una respuesta JSON
func JSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

// SuccessResponse respuesta exitosa
func SuccessResponse(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
	})
}

// ErrorResponse respuesta de error
func ErrorResponse(w http.ResponseWriter, status int, message string) {
	JSON(w, status, APIResponse{
		Success: false,
		Error:   message,
	})
}
