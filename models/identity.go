package models

type IdentityClaims interface {
	GetIssuer() string
	GetName() (string, string, bool)
	GetEmail() string
}

type FirebaseIdentity struct {
	Issuer string
	Email  string
}

func (i FirebaseIdentity) GetIssuer() string {
	return i.Issuer
}

func (i FirebaseIdentity) GetName() (string, string, bool) {
	return "", "", false
}

func (i FirebaseIdentity) GetEmail() string {
	return i.Email
}

type OidcIdentity struct {
	Issuer    string `json:"iss"`         //nolint:tagliatelle
	Firstname string `json:"given_name"`  //nolint:tagliatelle
	Lastname  string `json:"family_name"` //nolint:tagliatelle
	Email     string `json:"email"`
}

func (i OidcIdentity) GetIssuer() string {
	return i.Issuer
}

func (i OidcIdentity) GetName() (string, string, bool) {
	if i.Firstname != "" && i.Lastname != "" {
		return i.Firstname, i.Lastname, true
	}

	return "", "", false
}

func (c OidcIdentity) GetEmail() string {
	return c.Email
}

type ApiKeyIdentity struct {
	Issuer string
}

func (i ApiKeyIdentity) GetIssuer() string {
	return "marble"
}

func (i ApiKeyIdentity) GetName() (string, string, bool) {
	return "", "", false
}

func (i ApiKeyIdentity) GetEmail() string {
	return ""
}
