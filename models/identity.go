package models

type IdentityClaims interface {
	GetIssuer() string
	GetEmail() string
}

type FirebaseIdentity struct {
	Issuer string
	Email  string
}

func (i FirebaseIdentity) GetIssuer() string {
	return i.Issuer
}

func (i FirebaseIdentity) GetEmail() string {
	return i.Email
}

type OidcIdentity struct {
	Issuer string `json:"iss"` //nolint:tagliatelle
	Email  string `json:"email"`
}

func (i OidcIdentity) GetIssuer() string {
	return i.Issuer
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

func (i ApiKeyIdentity) GetEmail() string {
	return ""
}
