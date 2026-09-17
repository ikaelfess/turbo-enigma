package http

import (
	"net/http"
)

func NewRouter(orderHandler *OrderHandler) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /orders", orderHandler.CreateOrder)

	return mux
}

func health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
