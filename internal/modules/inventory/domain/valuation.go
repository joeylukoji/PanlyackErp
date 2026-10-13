package domain

import (
	"errors"
	"github.com/shopspring/decimal"
)

type Lot struct {
	ID       string
	Quantity decimal.Decimal
	Cost     decimal.Decimal
}

// Allocate never mutates its input. Lots must be ordered by receipt time.
func Allocate(lots []Lot, quantity decimal.Decimal, method string) (decimal.Decimal, []Lot, error) {
	if !quantity.IsPositive() {
		return decimal.Zero, nil, errors.New("quantity must be positive")
	}
	if method != "FIFO" && method != "AVCO" {
		return decimal.Zero, nil, errors.New("unknown valuation method")
	}
	total, value := decimal.Zero, decimal.Zero
	for _, lot := range lots {
		if lot.Quantity.IsNegative() || lot.Cost.IsNegative() {
			return decimal.Zero, nil, errors.New("invalid lot")
		}
		total = total.Add(lot.Quantity)
		value = value.Add(lot.Quantity.Mul(lot.Cost))
	}
	if total.LessThan(quantity) {
		return decimal.Zero, nil, errors.New("insufficient stock")
	}
	result := append([]Lot{}, lots...)
	remaining, cost := quantity, decimal.Zero
	for i := range result {
		used := decimal.Min(remaining, result[i].Quantity)
		cost = cost.Add(used.Mul(result[i].Cost))
		result[i].Quantity = result[i].Quantity.Sub(used)
		remaining = remaining.Sub(used)
	}
	if method == "AVCO" {
		average := value.Div(total)
		cost = quantity.Mul(average)
		for i := range result {
			result[i].Cost = average
		}
	}
	return cost.Round(4), result, nil
}
