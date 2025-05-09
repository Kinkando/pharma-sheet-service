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

var PharmaSheetMedicineBrands = newPharmaSheetMedicineBrandsTable("public", "pharma_sheet_medicine_brands", "")

type pharmaSheetMedicineBrandsTable struct {
	postgres.Table

	// Columns
	ID              postgres.ColumnString
	MedicationID    postgres.ColumnString
	TradeID         postgres.ColumnString
	TradeName       postgres.ColumnString
	BlisterImageURL postgres.ColumnString
	TabletImageURL  postgres.ColumnString
	BoxImageURL     postgres.ColumnString
	CreatedAt       postgres.ColumnTimestampz
	UpdatedAt       postgres.ColumnTimestampz

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type PharmaSheetMedicineBrandsTable struct {
	pharmaSheetMedicineBrandsTable

	EXCLUDED pharmaSheetMedicineBrandsTable
}

// AS creates new PharmaSheetMedicineBrandsTable with assigned alias
func (a PharmaSheetMedicineBrandsTable) AS(alias string) *PharmaSheetMedicineBrandsTable {
	return newPharmaSheetMedicineBrandsTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new PharmaSheetMedicineBrandsTable with assigned schema name
func (a PharmaSheetMedicineBrandsTable) FromSchema(schemaName string) *PharmaSheetMedicineBrandsTable {
	return newPharmaSheetMedicineBrandsTable(schemaName, a.TableName(), a.Alias())
}

// WithPrefix creates new PharmaSheetMedicineBrandsTable with assigned table prefix
func (a PharmaSheetMedicineBrandsTable) WithPrefix(prefix string) *PharmaSheetMedicineBrandsTable {
	return newPharmaSheetMedicineBrandsTable(a.SchemaName(), prefix+a.TableName(), a.TableName())
}

// WithSuffix creates new PharmaSheetMedicineBrandsTable with assigned table suffix
func (a PharmaSheetMedicineBrandsTable) WithSuffix(suffix string) *PharmaSheetMedicineBrandsTable {
	return newPharmaSheetMedicineBrandsTable(a.SchemaName(), a.TableName()+suffix, a.TableName())
}

func newPharmaSheetMedicineBrandsTable(schemaName, tableName, alias string) *PharmaSheetMedicineBrandsTable {
	return &PharmaSheetMedicineBrandsTable{
		pharmaSheetMedicineBrandsTable: newPharmaSheetMedicineBrandsTableImpl(schemaName, tableName, alias),
		EXCLUDED:                       newPharmaSheetMedicineBrandsTableImpl("", "excluded", ""),
	}
}

func newPharmaSheetMedicineBrandsTableImpl(schemaName, tableName, alias string) pharmaSheetMedicineBrandsTable {
	var (
		IDColumn              = postgres.StringColumn("id")
		MedicationIDColumn    = postgres.StringColumn("medication_id")
		TradeIDColumn         = postgres.StringColumn("trade_id")
		TradeNameColumn       = postgres.StringColumn("trade_name")
		BlisterImageURLColumn = postgres.StringColumn("blister_image_url")
		TabletImageURLColumn  = postgres.StringColumn("tablet_image_url")
		BoxImageURLColumn     = postgres.StringColumn("box_image_url")
		CreatedAtColumn       = postgres.TimestampzColumn("created_at")
		UpdatedAtColumn       = postgres.TimestampzColumn("updated_at")
		allColumns            = postgres.ColumnList{IDColumn, MedicationIDColumn, TradeIDColumn, TradeNameColumn, BlisterImageURLColumn, TabletImageURLColumn, BoxImageURLColumn, CreatedAtColumn, UpdatedAtColumn}
		mutableColumns        = postgres.ColumnList{MedicationIDColumn, TradeIDColumn, TradeNameColumn, BlisterImageURLColumn, TabletImageURLColumn, BoxImageURLColumn, CreatedAtColumn, UpdatedAtColumn}
	)

	return pharmaSheetMedicineBrandsTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		ID:              IDColumn,
		MedicationID:    MedicationIDColumn,
		TradeID:         TradeIDColumn,
		TradeName:       TradeNameColumn,
		BlisterImageURL: BlisterImageURLColumn,
		TabletImageURL:  TabletImageURLColumn,
		BoxImageURL:     BoxImageURLColumn,
		CreatedAt:       CreatedAtColumn,
		UpdatedAt:       UpdatedAtColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
