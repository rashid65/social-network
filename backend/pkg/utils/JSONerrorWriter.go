package utils

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

type SuccessResponse struct {
	Data   interface{} `json:"data`
	Status int         `json:"status"`
}

func WriteErrorJSON(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResp := ErrorResponse{
		Error:  http.StatusText(statusCode),
		Message: message,
		Status: statusCode,
	}

	json.NewEncoder(w).Encode(errorResp)
}

func WriteSuccessJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	succesResp := SuccessResponse{
		Data:   data,
		Status: statusCode,
	}

	json.NewEncoder(w).Encode(succesResp)
}