package ports

import "github.com/shopspring/decimal"

type Posted struct {
	OrganizationID, SourceID, Direction string
	Net, Cost, Paid, Due                decimal.Decimal
}

type Settled struct {
	OrganizationID, SourceID, Direction string
	Amount                              decimal.Decimal
}

type Summary struct {
	Revenue, Purchases, Cost, Cash, Inventory, Receivables, Payables string
	Count                                                            int
}
