package pages

import "github.com/shopspring/decimal"

type spendingSummary struct {
	Total, Share string
	Chart        bool
}

func spending(purchases, expenses string) spendingSummary {
	goods, goodsErr := decimal.NewFromString(purchases)
	charges, chargesErr := decimal.NewFromString(expenses)
	if goodsErr != nil || chargesErr != nil {
		return spendingSummary{Total: "—", Share: "0"}
	}
	total := goods.Add(charges)
	result := spendingSummary{Total: total.StringFixed(2), Share: "0"}
	if total.IsPositive() && !goods.IsNegative() && !charges.IsNegative() {
		result.Share = goods.Mul(decimal.NewFromInt(100)).Div(total).StringFixed(2)
		result.Chart = true
	}
	return result
}
