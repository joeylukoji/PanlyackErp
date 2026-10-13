package value

import (
	"errors"
	"github.com/shopspring/decimal"
	"regexp"
)

var decimalPattern = regexp.MustCompile(`^[0-9]{1,12}(\.[0-9]{1,6})?$`)

// Parse bounds decimal input before arithmetic or persistence.
func Parse(s string, scale int32) (decimal.Decimal, error) {
	if !decimalPattern.MatchString(s) {
		return decimal.Zero, errors.New("use a positive decimal number without separators")
	}
	d, err := decimal.NewFromString(s)
	if err != nil || d.Exponent() < -scale {
		return decimal.Zero, errors.New("decimal precision exceeds the allowed scale")
	}
	return d, nil
}

func Currency(s string) bool { return s == "USD" || s == "EUR" || s == "CDF" || s == "ZAR" }
