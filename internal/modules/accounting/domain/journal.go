package domain

import (
	"errors"
	"github.com/shopspring/decimal"
)

type Line struct {
	Role   string          `json:"role"`
	Debit  decimal.Decimal `json:"debit"`
	Credit decimal.Decimal `json:"credit"`
}
type Entry struct {
	Lines []Line `json:"lines"`
}
type Payment struct {
	Method string
	Amount decimal.Decimal
}
type Operation struct {
	Direction             string
	Net, Tax, Total, Cost decimal.Decimal
	Payments              []Payment
}

type PostingEngine interface {
	Build(Operation) (Entry, error)
}
type FrenchPCG struct{}
type USGAAP struct{}

func (FrenchPCG) Build(op Operation) (Entry, error) { return build(op) }
func (USGAAP) Build(op Operation) (Entry, error)    { return build(op) }

// Both policies use perpetual inventory; account codes are resolved per organization.
func build(op Operation) (Entry, error) {
	e := Entry{Lines: []Line{}}
	if !op.Net.Add(op.Tax).Equal(op.Total) || !op.Total.IsPositive() || op.Net.IsNegative() || op.Tax.IsNegative() || op.Cost.IsNegative() {
		return e, errors.New("invalid accounting amounts")
	}
	purchase := op.Direction == "PURCHASE"
	if !purchase && op.Direction != "SALE" {
		return e, errors.New("unknown accounting operation")
	}
	add := func(role string, amount decimal.Decimal, debit bool) {
		if amount.IsZero() {
			return
		}
		line := Line{Role: role}
		if debit {
			line.Debit = amount
		} else {
			line.Credit = amount
		}
		e.Lines = append(e.Lines, line)
	}
	paid := decimal.Zero
	for _, p := range op.Payments {
		switch p.Method {
		case "CASH", "BANK", "MOBILE_MONEY", "CARD":
		default:
			return e, errors.New("invalid cash role")
		}
		if p.Amount.IsNegative() {
			return e, errors.New("negative payment")
		}
		paid = paid.Add(p.Amount)
		add(p.Method, p.Amount, !purchase)
	}
	if paid.GreaterThan(op.Total) {
		return e, errors.New("overpayment")
	}
	if purchase {
		add("INVENTORY", op.Net, true)
		add("INPUT_TAX", op.Tax, true)
		add("ACCOUNTS_PAYABLE", op.Total.Sub(paid), false)
	} else {
		add("SALES", op.Net, false)
		add("OUTPUT_TAX", op.Tax, false)
		add("ACCOUNTS_RECEIVABLE", op.Total.Sub(paid), true)
		add("COGS", op.Cost, true)
		add("INVENTORY", op.Cost, false)
	}
	return e, e.Validate()
}

func (e Entry) Validate() error {
	if len(e.Lines) < 2 {
		return errors.New("journal requires at least two lines")
	}
	debit, credit := decimal.Zero, decimal.Zero
	for _, l := range e.Lines {
		if l.Role == "" || l.Debit.IsNegative() || l.Credit.IsNegative() {
			return errors.New("invalid journal line")
		}
		if l.Debit.IsPositive() == l.Credit.IsPositive() {
			return errors.New("journal line must contain only debit or credit")
		}
		debit = debit.Add(l.Debit)
		credit = credit.Add(l.Credit)
	}
	if !debit.Equal(credit) {
		return errors.New("journal is not balanced")
	}
	return nil
}

func (e Entry) Reverse() Entry {
	result := Entry{Lines: make([]Line, len(e.Lines))}
	for n, l := range e.Lines {
		result.Lines[n] = Line{Role: l.Role, Debit: l.Credit, Credit: l.Debit}
	}
	return result
}

func SettlementEntry(direction, method string, amount decimal.Decimal) (Entry, error) {
	entry := Entry{Lines: []Line{}}
	if !amount.IsPositive() {
		return entry, errors.New("settlement amount must be positive")
	}
	switch method {
	case "CASH", "BANK", "MOBILE_MONEY", "CARD":
	default:
		return entry, errors.New("invalid payment method")
	}
	switch direction {
	case "PURCHASE":
		entry.Lines = []Line{
			{Role: "ACCOUNTS_PAYABLE", Debit: amount},
			{Role: method, Credit: amount},
		}
	case "SALE":
		entry.Lines = []Line{
			{Role: method, Debit: amount},
			{Role: "ACCOUNTS_RECEIVABLE", Credit: amount},
		}
	default:
		return entry, errors.New("invalid settlement direction")
	}
	return entry, entry.Validate()
}
