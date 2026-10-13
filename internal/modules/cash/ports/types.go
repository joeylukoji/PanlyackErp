package ports

import "github.com/shopspring/decimal"

type Payment struct {
	Method string
	Amount decimal.Decimal
}

type Command struct {
	OrganizationID, SourceID, Direction string
	Payments                            []Payment
}

type Account struct{ Method, Balance string }

type Movement struct{ SourceID, Method, Amount, Date string }
