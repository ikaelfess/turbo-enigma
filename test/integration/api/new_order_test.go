package integration

import (
	"net/http"
	"testing"

	"github.com/andrei-polukhin/pgdbtemplate"
	"github.com/gavv/httpexpect/v2"
	"github.com/go-openapi/testify/v2/require"
	"github.com/google/uuid"
	"github.com/jinzhu/copier"
	"github.com/uptrace/bun"

	httpadapter "github.com/ikaelfess/transactional-outbox/internal/adapters/http"
	"github.com/ikaelfess/transactional-outbox/internal/adapters/postgres"
	"github.com/ikaelfess/transactional-outbox/internal/domain"
	"github.com/ikaelfess/transactional-outbox/test/integration/helpers"
)

func TestNewOrder(t *testing.T) {
	helpers.WithDatabaseContainer(t, func(t *testing.T, connectionString string) {
		helpers.WithDatabaseTemplateManager(t, connectionString, func(t *testing.T, tm *pgdbtemplate.TemplateManager) {
			tests := []struct {
				name               string
				requestBody        httpadapter.CreateOrderRequest
				expectedTotalCents int64
				statusCode         int
				expectedError      string
			}{
				{
					name: "valid order",
					requestBody: httpadapter.CreateOrderRequest{
						Items: []httpadapter.CreateOrderItem{
							{
								ItemName:       "Wireless Mouse",
								Quantity:       1,
								UnitPriceCents: 100,
							},
							{
								ItemName:       "USB-C Cable",
								Quantity:       2,
								UnitPriceCents: 400,
							},
						},
					},
					statusCode:         http.StatusCreated,
					expectedTotalCents: 900,
				},
				{
					name: "empty items",
					requestBody: httpadapter.CreateOrderRequest{
						Items: []httpadapter.CreateOrderItem{},
					},
					statusCode:    http.StatusUnprocessableEntity,
					expectedError: domain.ErrEmptyOrderItems.Error(),
				},
				{
					name: "invalid quantity",
					requestBody: httpadapter.CreateOrderRequest{
						Items: []httpadapter.CreateOrderItem{
							{
								ItemName:       "A",
								Quantity:       0,
								UnitPriceCents: 100,
							},
						},
					},
					statusCode:    http.StatusUnprocessableEntity,
					expectedError: domain.ErrInvalidQuantity.Error(),
				},
				{
					name: "negative unit price",
					requestBody: httpadapter.CreateOrderRequest{
						Items: []httpadapter.CreateOrderItem{
							{
								ItemName:       "A",
								Quantity:       1,
								UnitPriceCents: -100,
							},
						},
					},
					statusCode:    http.StatusUnprocessableEntity,
					expectedError: domain.ErrInvalidUnitPrice.Error(),
				},
				{
					name: "invalid item name",
					requestBody: httpadapter.CreateOrderRequest{
						Items: []httpadapter.CreateOrderItem{
							{
								ItemName:       "",
								Quantity:       1,
								UnitPriceCents: 100,
							},
						},
					},
					statusCode:    http.StatusUnprocessableEntity,
					expectedError: domain.ErrInvalidItemName.Error(),
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					helpers.WithDatabase(t, tm, func(t *testing.T, db *bun.DB) {
						helpers.WithApiServer(t, db, func(t *testing.T, apiServerUrl string) {
							e := httpexpect.WithConfig(httpexpect.Config{
								TestName: t.Name(),
								BaseURL:  apiServerUrl,
								Reporter: httpexpect.NewAssertReporter(t),
								Printers: nil,
							})

							response := e.POST("/orders").
								WithContext(t.Context()).
								WithJSON(tt.requestBody).
								Expect().
								Status(tt.statusCode)

							if tt.expectedError != "" {
								response.Body().Contains(tt.expectedError)
								return
							}

							responseBody := response.
								JSON().
								Object().
								ContainsKey("id").
								ContainsKey("total_cents")

							responseBody.
								Value("total_cents").
								Number().
								IsEqual(tt.expectedTotalCents)

							orderId := responseBody.
								Value("id").
								String().
								NotEmpty().
								Raw()

							t.Run("order", func(t *testing.T) {
								order, err := helpers.Find[postgres.Order](t, db, uuid.MustParse(orderId))
								require.NoError(t, err)
								require.Equal(t, tt.expectedTotalCents, order.TotalCents)
							})

							t.Run("order items", func(t *testing.T) {
								actualItems, err := helpers.FindAll[postgres.OrderItem](t, db)
								require.NoError(t, err)
								require.Len(t, actualItems, len(tt.requestBody.Items))

								var createOrderItems []httpadapter.CreateOrderItem
								copier.Copy(&createOrderItems, &actualItems)
								require.ElementsMatch(t, tt.requestBody.Items, createOrderItems)
							})

							t.Run("outbox event", func(t *testing.T) {
								event, err := helpers.FindBy[postgres.OutboxEvent](t, db, "aggregate_id = ?", orderId)
								require.NoError(t, err)
								require.NotEqual(t, uuid.Nil, event.ID)
								require.Equal(t, string(domain.OrderCreatedEventType), event.EventType)
								require.Nil(t, event.PublishedAt)

								domainEvent := event.ToDomain()
								require.Equal(t, orderId, domainEvent.Payload.ID.String())
								require.Equal(t, tt.expectedTotalCents, domainEvent.Payload.TotalCents)
								require.Len(t, domainEvent.Payload.Items, len(tt.requestBody.Items))

								for i, expectedItem := range tt.requestBody.Items {
									actualItem := domainEvent.Payload.Items[i]

									require.NotEqual(t, uuid.Nil, actualItem.ID)
									require.Equal(t, expectedItem.ItemName, actualItem.ItemName)
									require.Equal(t, expectedItem.Quantity, actualItem.Quantity)
									require.Equal(t, expectedItem.UnitPriceCents, actualItem.UnitPriceCents)
								}
							})
						})
					})
				})
			}
		})
	})
}
