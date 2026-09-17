package main

import (
	"go.uber.org/fx"

	"github.com/ikaelfess/transactional-outbox/internal/app"
)

func main() {
	fx.New(app.OutboxEventPublisherModule).Run()
}
