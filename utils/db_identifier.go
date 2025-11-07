package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

const (
	MAX_IDENTIFIER_LENGTH = 63
	HASH_LENGTH           = 6
)

// -1 because of the underscore before the hash
var IDENTIFIER_PREFIX_LENGTH = MAX_IDENTIFIER_LENGTH - HASH_LENGTH - 1

func TruncateIdentifier(name string) string {
	if len(name) <= MAX_IDENTIFIER_LENGTH {
		return name
	}
	return name[:IDENTIFIER_PREFIX_LENGTH] + "_" + hash(name)
}

func hash(name string) string {
	hash := sha256.Sum256([]byte(name))
	return hex.EncodeToString(hash[:])[:HASH_LENGTH]
}
