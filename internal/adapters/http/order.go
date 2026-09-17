package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ikaelfess/transactional-outbox/internal/domain"
	"github.com/ikaelfess/transactional-outbox/internal/usecase"
)

type CreateOrderRequest struct {
	Items []CreateOrderItem `json:"items"`
}

type CreateOrderItem struct {
	ItemName       string `json:"item_name"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
}

type CreateOrderResponse struct {
	ID         string `json:"id"`
	TotalCents int64  `json:"total_cents"`
}

var _ OrderUsecase = (*usecase.OrderUsecase)(nil)

type OrderUsecase interface {
	CreateOrder(ctx context.Context, items []domain.OrderItem) (domain.Order, error)
}

type OrderHandler struct {
	usecase OrderUsecase
}

func NewOrderHandler(usecase OrderUsecase) *OrderHandler {
	return &OrderHandler{usecase: usecase}
}

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	items := make([]domain.OrderItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, domain.OrderItem{
			ItemName:       item.ItemName,
			Quantity:       item.Quantity,
			UnitPriceCents: item.UnitPriceCents,
		})
	}

	order, err := h.usecase.CreateOrder(r.Context(), items)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	response := CreateOrderResponse{
		ID:         order.ID.String(),
		TotalCents: order.TotalCents,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	_ = json.NewEncoder(w).Encode(response)
}
