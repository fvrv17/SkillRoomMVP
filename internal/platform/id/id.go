package id

import (
	"crypto/rand"
	"encoding/hex"
)

func New(prefix string) string {
	bytes := make([]byte, 10)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(bytes)
}
