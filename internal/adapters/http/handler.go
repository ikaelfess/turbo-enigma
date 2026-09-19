package http

import (
	"net/http"

	"github.com/justinas/alice"
	"github.com/rs/zerolog"
)

func NewHTTPHandler(logger zerolog.Logger, router *http.ServeMux) http.Handler {
	return alice.New(
		Recover(),
		Telemetry(),
		RequestID(),
		Logger(logger),
	).Then(router)
}
