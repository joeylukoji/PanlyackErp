package ports

import "github.com/shopspring/decimal"

type Posted struct {
	Expense                             bool
	OrganizationID, SourceID, Direction string
	Net, Cost, Paid, Due                decimal.Decimal
}

type Settled struct {
	OrganizationID, SourceID, Direction string
	Amount                              decimal.Decimal
}

type Summary struct {
	Revenue, Purchases, Expenses, Cost, Cash, Inventory, Receivables, Payables string
	Count                                                                      int
}
