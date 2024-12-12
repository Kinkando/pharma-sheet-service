package generator

import "github.com/google/uuid"

func UUID() string {
	newUUID, _ := uuid.NewV7()
	return newUUID.String()
}
