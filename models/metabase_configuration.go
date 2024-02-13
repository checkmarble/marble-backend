package models

type MetabaseConfiguration struct {
	SiteUrl             string
	JwtSigningKey       []byte
	TokenLifetimeMinute int
	Resources           map[EmbeddingType]int
}
