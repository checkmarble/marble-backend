package models

type IdentityClaims interface {
	GetIssuer() string
	GetEmail() string
	GetProfile() *IdentityUpdatableClaims
}

type IdentityUpdatableClaims struct {
	Firstname string
	Lastname  string
	Picture   string
}

type FirebaseIdentity struct {
	Issuer  string
	Email   string
	Picture string
}

func (i FirebaseIdentity) GetIssuer() string {
	return i.Issuer
}

func (i FirebaseIdentity) GetProfile() *IdentityUpdatableClaims {
	return &IdentityUpdatableClaims{
		Picture: i.Picture,
	}
}

func (i FirebaseIdentity) GetEmail() string {
	return i.Email
}

type OidcIdentity struct {
	Issuer        string `json:"iss"`         //nolint:tagliatelle
	Firstname     string `json:"given_name"`  //nolint:tagliatelle
	Lastname      string `json:"family_name"` //nolint:tagliatelle
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Picture       string `json:"picture"`

	SkipEmailVerify bool `json:"-"`
}

func (i OidcIdentity) GetIssuer() string {
	return i.Issuer
}

func (i OidcIdentity) GetProfile() *IdentityUpdatableClaims {
	return &IdentityUpdatableClaims{
		Firstname: i.Firstname,
		Lastname:  i.Lastname,
		Picture:   i.Picture,
	}
}

func (i OidcIdentity) GetEmail() string {
	if !i.SkipEmailVerify && !i.EmailVerified {
		return ""
	}
	return i.Email
}

type ApiKeyIdentity struct {
	Issuer string
}

func (i ApiKeyIdentity) GetIssuer() string {
	return "marble"
}

func (i ApiKeyIdentity) GetProfile() *IdentityUpdatableClaims {
	return nil
}

func (i ApiKeyIdentity) GetEmail() string {
	return ""
}
