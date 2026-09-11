package domain

import (
	"errors"

	"github.com/google/uuid"
)

var (
	ErrEmptyOrderItems  = errors.New("order must contain at least one item")
	ErrInvalidItemName  = errors.New("item name must not be empty")
	ErrInvalidQuantity  = errors.New("quantity must be greater than zero")
	ErrInvalidUnitPrice = errors.New("unit price must not be negative")
)

type Order struct {
	ID         uuid.UUID
	Items      []OrderItem
	TotalCents int64
}

type OrderItem struct {
	ID             uuid.UUID
	ItemName       string
	Quantity       int
	UnitPriceCents int64
}

func NewOrder(items []OrderItem) (Order, error) {
	if len(items) == 0 {
		return Order{}, ErrEmptyOrderItems
	}

	var totalCents int64
	for _, item := range items {
		if err := item.Validate(); err != nil {
			return Order{}, err
		}

		totalCents += item.TotalCents()
	}

	return Order{
		Items:      items,
		TotalCents: totalCents,
	}, nil
}

func (i OrderItem) Validate() error {
	if i.ItemName == "" {
		return ErrInvalidItemName
	}

	if i.Quantity <= 0 {
		return ErrInvalidQuantity
	}

	if i.UnitPriceCents < 0 {
		return ErrInvalidUnitPrice
	}

	return nil
}

func (i OrderItem) TotalCents() int64 {
	return int64(i.Quantity) * i.UnitPriceCents
}
