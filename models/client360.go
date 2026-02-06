package models

type Client360Table struct {
	Table

	IndexReady bool
}

type Client360SearchInput struct {
	Table string
	Terms string
	Page  uint64
}
