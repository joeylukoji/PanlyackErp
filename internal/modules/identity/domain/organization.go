package domain

import "errors"

type Organization struct {
	ID                 string
	Name               string
	Currency           string
	AccountingStandard string
	InventoryMethod    string
}

func (o Organization) Validate() error {
	if o.AccountingStandard != "FR_PCG" && o.AccountingStandard != "US_GAAP" {
		return errors.New("unknown accounting standard")
	}
	if o.InventoryMethod != "FIFO" && o.InventoryMethod != "AVCO" {
		return errors.New("unknown inventory method")
	}
	return nil
}
