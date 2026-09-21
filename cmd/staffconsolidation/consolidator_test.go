package staffconsolidation

import "testing"

func TestCanonicalEmail(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{"Alice+Acme@checkmarble.com", "alice@checkmarble.com", true},
		{"alice+globex@checkmarble.com", "alice@checkmarble.com", true},
		{"alice@checkmarble.com", "", false},
		{"alice+client@example.com", "", false},
	}
	for _, test := range tests {
		got, ok := canonicalEmail(test.input)
		if got != test.want || ok != test.ok {
			t.Errorf("canonicalEmail(%q) = %q, %t; want %q, %t", test.input, got, ok, test.want, test.ok)
		}
	}
}

func TestChooseAliasCanonicalPrefersActiveAliases(t *testing.T) {
	canonical, ok := chooseAliasCanonical([]userRow{
		{id: "00000000-0000-0000-0000-000000000001", email: "alice+old@checkmarble.com", deleted: true},
		{id: "00000000-0000-0000-0000-000000000002", email: "alice+active@checkmarble.com"},
	})
	if !ok {
		t.Fatal("expected an alias canonical")
	}
	if canonical.email != "alice+active@checkmarble.com" {
		t.Fatalf("selected %q, want active alias", canonical.email)
	}
}
