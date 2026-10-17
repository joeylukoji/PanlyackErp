// Command accounting-examples exports synthetic role labels from the existing posting engine.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/shopspring/decimal"
	accounting "panlyackerp/internal/modules/accounting/domain"
)

type facts struct {
	Operation      string   `json:"operation"`
	Settlement     string   `json:"settlement"`
	PaymentMethods []string `json:"payment_methods"`
	Products       []string `json:"products"`
	CostRecognized bool     `json:"cost_recognized"`
	TaxTreatment   string   `json:"tax_treatment"`
}

type roles struct {
	Debit  []string `json:"debit_roles"`
	Credit []string `json:"credit_roles"`
}

type example struct {
	ID         string `json:"id"`
	Split      string `json:"split"`
	Facts      facts  `json:"facts"`
	Target     roles  `json:"target"`
	Provenance string `json:"provenance"`
}

func examples() ([]example, error) {
	result := []example{}
	products := []string{"Bulk rice", "Office paper", "Cleaning soap", "Work gloves", "Coffee supplies", "Packaging boxes"}
	for _, operation := range []string{"INVENTORY_PURCHASE", "OPERATING_EXPENSE", "SALE"} {
		costs := []bool{false}
		if operation == "SALE" {
			costs = append(costs, true)
		}
		for _, cost := range costs {
			for _, taxed := range []bool{false, true} {
				for _, settlement := range []string{"UNPAID", "PARTIAL", "PAID"} {
					methods := []string{""}
					if settlement != "UNPAID" {
						methods = []string{"CASH", "BANK"}
					}
					for _, method := range methods {
						for index, product := range products {
							input := facts{Operation: operation, Settlement: settlement, PaymentMethods: []string{}, Products: []string{product}, CostRecognized: cost}
							input.TaxTreatment = "NONE"
							op := accounting.Operation{Direction: "PURCHASE", Expense: operation == "OPERATING_EXPENSE", Net: decimal.NewFromInt(100), Total: decimal.NewFromInt(100), Payments: []accounting.Payment{}}
							if taxed {
								input.TaxTreatment = "NONRECOVERABLE_PURCHASE_TAX"
								op.Tax = decimal.NewFromInt(8)
								op.Total = op.Net.Add(op.Tax)
								if operation == "SALE" {
									input.TaxTreatment = "BUYER_SALES_TAX"
								}
							}
							if operation == "SALE" {
								op.Direction = "SALE"
							}
							if cost {
								op.Cost = decimal.NewFromInt(30)
							}
							if method != "" {
								input.PaymentMethods = append(input.PaymentMethods, method)
								amount := decimal.NewFromInt(40)
								if settlement == "PAID" {
									amount = op.Total
								}
								op.Payments = append(op.Payments, accounting.Payment{Method: method, Amount: amount})
							}
							entry, err := (accounting.USGAAP{}).Build(op)
							if err != nil {
								return nil, err
							}
							target := roles{Debit: []string{}, Credit: []string{}}
							for _, line := range entry.Lines {
								if line.Debit.IsPositive() {
									target.Debit = append(target.Debit, line.Role)
								}
								if line.Credit.IsPositive() {
									target.Credit = append(target.Credit, line.Role)
								}
							}
							sort.Strings(target.Debit)
							sort.Strings(target.Credit)
							split := "train"
							if index == 4 {
								split = "validation"
							}
							if index == 5 {
								split = "test"
							}
							result = append(result, example{ID: fmt.Sprintf("us-v2-%03d", len(result)+1), Split: split, Facts: input, Target: target, Provenance: "synthetic; Go US perpetual-inventory engine; explicit tax treatment, illustrative amount not jurisdiction rate; no accountant review"})
						}
					}
				}
			}
		}
	}
	for _, operation := range []string{"SUPPLIER_PAYMENT", "CUSTOMER_RECEIPT", "CASH_TO_BANK", "BANK_TO_CASH"} {
		methods := []string{"CASH", "BANK"}
		if operation == "CASH_TO_BANK" || operation == "BANK_TO_CASH" {
			methods = []string{"TRANSFER"}
		}
		for _, method := range methods {
			for index, product := range products {
				input := facts{Operation: operation, Settlement: "PAID", Products: []string{product}, PaymentMethods: []string{method}, TaxTreatment: "NONE"}
				var entry accounting.Entry
				var err error
				switch operation {
				case "SUPPLIER_PAYMENT":
					entry, err = accounting.SettlementEntry("PURCHASE", method, decimal.NewFromInt(40))
				case "CUSTOMER_RECEIPT":
					entry, err = accounting.SettlementEntry("SALE", method, decimal.NewFromInt(40))
				default:
					input.PaymentMethods = []string{"CASH", "BANK"}
					debit, credit := "BANK", "CASH"
					if operation == "BANK_TO_CASH" {
						debit, credit = credit, debit
					}
					entry = accounting.Entry{Lines: []accounting.Line{{Role: debit, Debit: decimal.NewFromInt(40)}, {Role: credit, Credit: decimal.NewFromInt(40)}}}
					err = entry.Validate()
				}
				if err != nil {
					return nil, err
				}
				target := roles{Debit: []string{}, Credit: []string{}}
				for _, line := range entry.Lines {
					if line.Debit.IsPositive() {
						target.Debit = append(target.Debit, line.Role)
					}
					if line.Credit.IsPositive() {
						target.Credit = append(target.Credit, line.Role)
					}
				}
				split := "train"
				if index == 4 {
					split = "validation"
				}
				if index == 5 {
					split = "test"
				}
				result = append(result, example{ID: fmt.Sprintf("us-v2-%03d", len(result)+1), Split: split, Facts: input, Target: target, Provenance: "synthetic Go settlement or balanced own-funds transfer; no accountant review"})
			}
		}
	}
	return result, nil
}

func main() {
	output := flag.String("output", "", "New JSON file; existing files are never overwritten")
	flag.Parse()
	rows, err := examples()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	file, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(rows); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %d synthetic examples to %s\n", len(rows), *output)
}
