package util

import (
	"unicode"
)

func CamelToSnake(camel string) string {
	var snake string
	for i, char := range camel {
		if unicode.IsUpper(char) && i > 0 {
			snake += "_" // Add underscore before uppercase letters, except at the start
		}
		snake += string(unicode.ToLower(char))
	}
	return snake
}
