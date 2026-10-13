package application

import (
	"context"
	"errors"
	"github.com/shopspring/decimal"
	"panlyackerp/internal/modules/inventory/domain"
	"strings"
)

type Item struct {
	SKU, Name, Unit string
	Quantity, Price decimal.Decimal
}
type Product struct {
	ID, SKU, Name, Unit string
	Quantity, Value     decimal.Decimal
}
type Command struct {
	OrganizationID, SourceID, Direction, Method string
	Items                                       []Item
}
type Repository interface {
	FindOrCreate(context.Context, string, Item, bool) (Product, error)
	Lots(context.Context, string, string) ([]domain.Lot, error)
	Receive(context.Context, string, string, Product, Item) error
	Issue(context.Context, string, string, Product, decimal.Decimal, decimal.Decimal, []domain.Lot) error
}
type Service struct{ Repository Repository }

func (s Service) Apply(ctx context.Context, c Command) (decimal.Decimal, error) {
	cost := decimal.Zero
	for _, item := range c.Items {
		if item.SKU == "" {
			item.SKU = strings.ToUpper(strings.Join(strings.Fields(item.Name), "-"))
		}
		p, err := s.Repository.FindOrCreate(ctx, c.OrganizationID, item, c.Direction == "PURCHASE")
		if err != nil {
			return cost, err
		}
		if p.Unit != item.Unit {
			return cost, errors.New("product unit differs from existing inventory")
		}
		if c.Direction == "PURCHASE" {
			if err := s.Repository.Receive(ctx, c.OrganizationID, c.SourceID, p, item); err != nil {
				return cost, err
			}
			continue
		}
		lots, err := s.Repository.Lots(ctx, c.OrganizationID, p.ID)
		if err != nil {
			return cost, err
		}
		used, remaining, err := domain.Allocate(lots, item.Quantity, c.Method)
		if err != nil {
			return cost, err
		}
		// Consume the remaining book value on the final issue to avoid rounding residue.
		if p.Quantity.Equal(item.Quantity) {
			used = p.Value
		}
		if err := s.Repository.Issue(ctx, c.OrganizationID, c.SourceID, p, item.Quantity, used, remaining); err != nil {
			return cost, err
		}
		cost = cost.Add(used)
	}
	return cost, nil
}
