package auth

import "testing"

func TestIsEmailInAllowedDomain(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{name: "accepts exact domain", email: "alice@example.test", want: true},
		{name: "accepts domain case insensitively", email: "alice@EXAMPLE.TEST", want: true},
		{name: "rejects subdomain", email: "alice@division.example.test"},
		{name: "rejects lookalike", email: "alice@notexample.test"},
		{name: "rejects domain suffix", email: "alice@example.test.evil"},
		{name: "rejects missing local part", email: "@example.test"},
		{name: "rejects missing domain", email: "alice@"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmailInAllowedDomain(tt.email, []string{"@example.test"}, true); got != tt.want {
				t.Errorf("isEmailInAllowedDomain(%q) = %t, want %t", tt.email, got, tt.want)
			}
		})
	}
}

func TestIsEmailInAllowedDomain_GlobalOIDCKeepsSuffixMatching(t *testing.T) {
	if !isEmailInAllowedDomain("alice@example.test", []string{"@example.test"}, false) {
		t.Error("global OIDC domain matching should retain suffix semantics")
	}
}
