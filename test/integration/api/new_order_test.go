package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/andrei-polukhin/pgdbtemplate"
	"github.com/gavv/httpexpect/v2"
	"github.com/go-openapi/testify/v2/require"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	apphttp "github.com/ikaelfess/transactional-outbox/internal/adapters/http"
	"github.com/ikaelfess/transactional-outbox/internal/domain"
	"github.com/ikaelfess/transactional-outbox/test/integration/helpers"
)

func TestNewOrder(t *testing.T) {
	helpers.WithDatabaseContainer(t, func(t *testing.T, connectionString string) {
		helpers.WithDatabaseTemplateManager(t, connectionString, func(t *testing.T, tm *pgdbtemplate.TemplateManager) {
			tests := []struct {
				name               string
				requestBody        apphttp.CreateOrderRequest
				expectedTotalCents int64
				statusCode         int
				expectedError      string
			}{
				{
					name: "valid order",
					requestBody: apphttp.CreateOrderRequest{
						Items: []apphttp.CreateOrderItem{
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
					requestBody: apphttp.CreateOrderRequest{
						Items: []apphttp.CreateOrderItem{},
					},
					statusCode:    http.StatusUnprocessableEntity,
					expectedError: domain.ErrEmptyOrderItems.Error(),
				},
				{
					name: "invalid quantity",
					requestBody: apphttp.CreateOrderRequest{
						Items: []apphttp.CreateOrderItem{
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
					requestBody: apphttp.CreateOrderRequest{
						Items: []apphttp.CreateOrderItem{
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
					requestBody: apphttp.CreateOrderRequest{
						Items: []apphttp.CreateOrderItem{
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

					helpers.WithDatabase(t, tm, func(t *testing.T, db *pgxpool.Pool) {
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
								var totalCents int64
								err := db.QueryRow(
									t.Context(),
									`SELECT total_cents FROM orders WHERE id = $1`,
									orderId,
								).Scan(&totalCents)

								require.NoError(t, err)
								require.Equal(t, tt.expectedTotalCents, totalCents)
							})

							t.Run("order items", func(t *testing.T) {
								rows, err := db.Query(
									t.Context(),
									`
										SELECT item_name, quantity, unit_price_cents
										FROM order_items
										WHERE order_id = $1 ORDER BY item_name
									`,
									orderId,
								)
								require.NoError(t, err)
								t.Cleanup(func() {
									rows.Close()
								})

								var orderItems []apphttp.CreateOrderItem
								for rows.Next() {
									var item apphttp.CreateOrderItem
									err := rows.Scan(
										&item.ItemName,
										&item.Quantity,
										&item.UnitPriceCents,
									)
									require.NoError(t, err)
									orderItems = append(orderItems, item)
								}

								require.NoError(t, rows.Err())
								require.ElementsMatch(t, tt.requestBody.Items, orderItems)
							})

							t.Run("outbox event", func(t *testing.T) {
								var event domain.OutboxEvent
								var eventPayload []byte
								err := db.QueryRow(
									t.Context(),
									`
										SELECT id, aggregate_id, event_type, payload, published_at
										FROM outbox_events
										WHERE aggregate_id = $1
									`,
									orderId,
								).Scan(
									&event.ID,
									&event.AggregateID,
									&event.EventType,
									&eventPayload,
									&event.PublishedAt,
								)
								require.NoError(t, err)

								require.NotEqual(t, uuid.Nil, event.ID)
								require.Equal(t, orderId, event.AggregateID.String())
								require.Equal(t, domain.OrderCreatedEventType, event.EventType)
								require.Nil(t, event.PublishedAt)

								err = json.Unmarshal(eventPayload, &event.Payload)
								require.NoError(t, err)

								require.Equal(t, orderId, event.Payload.ID.String())
								require.Equal(t, tt.expectedTotalCents, event.Payload.TotalCents)

								require.Len(t, event.Payload.Items, len(tt.requestBody.Items))
								for i, expectedItem := range tt.requestBody.Items {
									actualItem := event.Payload.Items[i]

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
