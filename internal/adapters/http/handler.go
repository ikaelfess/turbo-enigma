package http

import (
	"net/http"

	"github.com/justinas/alice"
	"github.com/rs/zerolog"

	"github.com/ikaelfess/transactional-outbox/internal/usecase"
)

type Handler struct {
	orderService *usecase.OrderService
}

func NewHandler(orderService *usecase.OrderService) *Handler {
	return &Handler{orderService: orderService}
}

func NewHTTPHandler(logger zerolog.Logger, router *http.ServeMux) http.Handler {
	return alice.New(
		RequestId(),
		Logger(logger),
		Recover(),
	).Then(router)
}
