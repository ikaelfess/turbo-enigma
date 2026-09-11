package http

import (
	"net/http"
)

func NewRouter(handler *Handler) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /orders", handler.CreateOrder)

	return mux
}

func health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
