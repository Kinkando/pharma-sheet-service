//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package table

import (
	"github.com/go-jet/jet/v2/postgres"
)

var Medicines = newMedicinesTable("public", "medicines", "")

type medicinesTable struct {
	postgres.Table

	// Columns
	MedicineID  postgres.ColumnString
	WarehouseID postgres.ColumnString
	LockerID    postgres.ColumnString
	Floor       postgres.ColumnInteger
	No          postgres.ColumnInteger
	Address     postgres.ColumnString
	Description postgres.ColumnString
	MedicalName postgres.ColumnString
	Label       postgres.ColumnString
	ImageURL    postgres.ColumnString
	CreatedAt   postgres.ColumnTimestampz
	UpdatedAt   postgres.ColumnTimestampz

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type MedicinesTable struct {
	medicinesTable

	EXCLUDED medicinesTable
}

// AS creates new MedicinesTable with assigned alias
func (a MedicinesTable) AS(alias string) *MedicinesTable {
	return newMedicinesTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new MedicinesTable with assigned schema name
func (a MedicinesTable) FromSchema(schemaName string) *MedicinesTable {
	return newMedicinesTable(schemaName, a.TableName(), a.Alias())
}

// WithPrefix creates new MedicinesTable with assigned table prefix
func (a MedicinesTable) WithPrefix(prefix string) *MedicinesTable {
	return newMedicinesTable(a.SchemaName(), prefix+a.TableName(), a.TableName())
}

// WithSuffix creates new MedicinesTable with assigned table suffix
func (a MedicinesTable) WithSuffix(suffix string) *MedicinesTable {
	return newMedicinesTable(a.SchemaName(), a.TableName()+suffix, a.TableName())
}

func newMedicinesTable(schemaName, tableName, alias string) *MedicinesTable {
	return &MedicinesTable{
		medicinesTable: newMedicinesTableImpl(schemaName, tableName, alias),
		EXCLUDED:       newMedicinesTableImpl("", "excluded", ""),
	}
}

func newMedicinesTableImpl(schemaName, tableName, alias string) medicinesTable {
	var (
		MedicineIDColumn  = postgres.StringColumn("medicine_id")
		WarehouseIDColumn = postgres.StringColumn("warehouse_id")
		LockerIDColumn    = postgres.StringColumn("locker_id")
		FloorColumn       = postgres.IntegerColumn("floor")
		NoColumn          = postgres.IntegerColumn("no")
		AddressColumn     = postgres.StringColumn("address")
		DescriptionColumn = postgres.StringColumn("description")
		MedicalNameColumn = postgres.StringColumn("medical_name")
		LabelColumn       = postgres.StringColumn("label")
		ImageURLColumn    = postgres.StringColumn("image_url")
		CreatedAtColumn   = postgres.TimestampzColumn("created_at")
		UpdatedAtColumn   = postgres.TimestampzColumn("updated_at")
		allColumns        = postgres.ColumnList{MedicineIDColumn, WarehouseIDColumn, LockerIDColumn, FloorColumn, NoColumn, AddressColumn, DescriptionColumn, MedicalNameColumn, LabelColumn, ImageURLColumn, CreatedAtColumn, UpdatedAtColumn}
		mutableColumns    = postgres.ColumnList{WarehouseIDColumn, LockerIDColumn, FloorColumn, NoColumn, AddressColumn, DescriptionColumn, MedicalNameColumn, LabelColumn, ImageURLColumn, CreatedAtColumn, UpdatedAtColumn}
	)

	return medicinesTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		MedicineID:  MedicineIDColumn,
		WarehouseID: WarehouseIDColumn,
		LockerID:    LockerIDColumn,
		Floor:       FloorColumn,
		No:          NoColumn,
		Address:     AddressColumn,
		Description: DescriptionColumn,
		MedicalName: MedicalNameColumn,
		Label:       LabelColumn,
		ImageURL:    ImageURLColumn,
		CreatedAt:   CreatedAtColumn,
		UpdatedAt:   UpdatedAtColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}