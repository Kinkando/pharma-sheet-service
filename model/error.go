package model

import (
	"errors"
	"strings"
)

var (
	ErrResourceNotAllowed = errors.New("resource not allowed")
)

func IsConflictError(err error) bool {
	return strings.Contains(err.Error(), "duplicate key value violates unique constraint")
}
