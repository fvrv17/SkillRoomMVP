package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxBodyBytes = 1 << 20

type ErrorResponse struct {
	Error string `json:"error"`
}

func DecodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()

	limited := http.MaxBytesReader(nil, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}

	if err := decoder.Decode(&struct{}{}); err != nil && !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON value")
	}

	return nil
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message})
}
