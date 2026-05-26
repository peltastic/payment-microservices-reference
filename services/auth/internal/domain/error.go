package domain

import "errors"

var (
	ErrNotFound   = errors.New("not found")
	ErrKeyRevoked = errors.New("key is revoked")
	ErrInvalidKey = errors.New("invalid key")
)
