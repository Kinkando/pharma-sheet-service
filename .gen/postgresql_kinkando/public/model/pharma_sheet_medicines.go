//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package model

import (
	"github.com/google/uuid"
	"time"
)

type PharmaSheetMedicines struct {
	MedicineID  uuid.UUID `sql:"primary_key"`
	WarehouseID uuid.UUID
	LockerID    uuid.UUID
	Floor       int32
	No          int32
	Address     string
	Description string
	MedicalName string
	Label       string
	ImageURL    *string
	CreatedAt   time.Time
	UpdatedAt   *time.Time
}