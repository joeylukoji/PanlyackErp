package main

import (
	"testing"
)

func TestSyntheticExamplesSeparateProductReferencesAndUseBalancedEngineRoles(t *testing.T) {
	rows, err := examples()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	seen := map[string]string{}
	for _, row := range rows {
		counts[row.Split]++
		product := row.Facts.Products[0]
		if split, ok := seen[product]; ok && split != row.Split {
			t.Fatal("product leaked between partitions")
		}
		seen[product] = row.Split
		if len(row.Target.Debit) == 0 || len(row.Target.Credit) == 0 {
			t.Fatal("missing posting sides")
		}
		if row.Facts.Operation == "OPERATING_EXPENSE" {
			for _, role := range row.Target.Debit {
				if role == "INVENTORY" {
					t.Fatal("expense creates inventory")
				}
			}
		}
	}
	if counts["train"] != 184 || counts["validation"] != 46 || counts["test"] != 46 {
		t.Fatal(counts)
	}
}
