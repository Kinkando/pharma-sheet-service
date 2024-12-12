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

var Lockers = newLockersTable("public", "lockers", "")

type lockersTable struct {
	postgres.Table

	// Columns
	LockerID    postgres.ColumnString
	WarehouseID postgres.ColumnString
	Name        postgres.ColumnString
	CreatedAt   postgres.ColumnTimestampz
	UpdatedAt   postgres.ColumnTimestampz

	AllColumns     postgres.ColumnList
	MutableColumns postgres.ColumnList
}

type LockersTable struct {
	lockersTable

	EXCLUDED lockersTable
}

// AS creates new LockersTable with assigned alias
func (a LockersTable) AS(alias string) *LockersTable {
	return newLockersTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new LockersTable with assigned schema name
func (a LockersTable) FromSchema(schemaName string) *LockersTable {
	return newLockersTable(schemaName, a.TableName(), a.Alias())
}

// WithPrefix creates new LockersTable with assigned table prefix
func (a LockersTable) WithPrefix(prefix string) *LockersTable {
	return newLockersTable(a.SchemaName(), prefix+a.TableName(), a.TableName())
}

// WithSuffix creates new LockersTable with assigned table suffix
func (a LockersTable) WithSuffix(suffix string) *LockersTable {
	return newLockersTable(a.SchemaName(), a.TableName()+suffix, a.TableName())
}

func newLockersTable(schemaName, tableName, alias string) *LockersTable {
	return &LockersTable{
		lockersTable: newLockersTableImpl(schemaName, tableName, alias),
		EXCLUDED:     newLockersTableImpl("", "excluded", ""),
	}
}

func newLockersTableImpl(schemaName, tableName, alias string) lockersTable {
	var (
		LockerIDColumn    = postgres.StringColumn("locker_id")
		WarehouseIDColumn = postgres.StringColumn("warehouse_id")
		NameColumn        = postgres.StringColumn("name")
		CreatedAtColumn   = postgres.TimestampzColumn("created_at")
		UpdatedAtColumn   = postgres.TimestampzColumn("updated_at")
		allColumns        = postgres.ColumnList{LockerIDColumn, WarehouseIDColumn, NameColumn, CreatedAtColumn, UpdatedAtColumn}
		mutableColumns    = postgres.ColumnList{WarehouseIDColumn, NameColumn, CreatedAtColumn, UpdatedAtColumn}
	)

	return lockersTable{
		Table: postgres.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		LockerID:    LockerIDColumn,
		WarehouseID: WarehouseIDColumn,
		Name:        NameColumn,
		CreatedAt:   CreatedAtColumn,
		UpdatedAt:   UpdatedAtColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}