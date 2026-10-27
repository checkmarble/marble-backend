package cmd

import "testing"

func TestHasGlobalOIDCConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		issuer   string
		clientID string
		want     bool
	}{
		{name: "no configuration"},
		{name: "issuer", issuer: "http://localhost:5556/dex", want: true},
		{name: "client ID", clientID: "marble-app", want: true},
		{name: "complete configuration", issuer: "http://localhost:5556/dex", clientID: "marble-app", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AUTH_OIDC_ISSUER", tt.issuer)
			t.Setenv("AUTH_OIDC_CLIENT_ID", tt.clientID)

			if got := hasGlobalOIDCConfiguration(); got != tt.want {
				t.Errorf("hasGlobalOIDCConfiguration() = %t, want %t", got, tt.want)
			}
		})
	}
}
