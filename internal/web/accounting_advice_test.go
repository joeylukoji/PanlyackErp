package web

import "testing"

func TestRoleComparisonRejectsAddedMissingAndRepeatedModelAccounts(t *testing.T) {
	expected := []string{"CASH", "ACCOUNTS_PAYABLE"}
	if !sameRoles(expected, []string{"ACCOUNTS_PAYABLE", "CASH"}) {
		t.Fatal("role order changed meaning")
	}
	for _, invalid := range [][]string{{"CASH"}, {"CASH", "CASH"}, {"CASH", "ACCOUNTS_PAYABLE", "BANK"}} {
		if sameRoles(expected, invalid) {
			t.Fatal("accepted different posting roles", invalid)
		}
	}
}
