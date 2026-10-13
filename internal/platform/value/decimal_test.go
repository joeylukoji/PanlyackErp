package value

import "testing"

func TestExactDecimal(t *testing.T) {
	a, err := Parse("0.1", 4)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Parse("0.2", 4)
	if a.Add(b).String() != "0.3" {
		t.Fatal("money lost precision")
	}
	for _, s := range []string{"NaN", "-1", "1e100", "0.00001", ""} {
		if _, err := Parse(s, 4); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
