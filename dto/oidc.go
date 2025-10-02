package dto

import (
	"time"

	"golang.org/x/oauth2"
)

type OidcTokens struct {
	TokenType    string    `json:"token_type"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	IdToken      string    `json:"id_token"`
	Expiry       time.Time `json:"expiry"`
	ExpiresIn    int64     `json:"expires_in"`
}

func AdaptOidcTokens(tokens *oauth2.Token) OidcTokens {
	idToken := ""
	if tok, ok := tokens.Extra("id_token").(string); ok {
		idToken = tok
	}

	return OidcTokens{
		TokenType:    tokens.TokenType,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		IdToken:      idToken,
		Expiry:       tokens.Expiry,
		ExpiresIn:    tokens.ExpiresIn,
	}
}
