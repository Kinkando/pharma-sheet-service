//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package enum

import "github.com/go-jet/jet/v2/postgres"

var ApprovalStatus = &struct {
	Approved postgres.StringExpression
	Pending  postgres.StringExpression
}{
	Approved: postgres.NewEnumValue("APPROVED"),
	Pending:  postgres.NewEnumValue("PENDING"),
}
