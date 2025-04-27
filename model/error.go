package model

import "strings"

func IsConflictError(err error) bool {
	return strings.Contains(err.Error(), "duplicate key value violates unique constraint")
}
