//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package model

import "errors"

type PharmaSheetRole string

const (
	PharmaSheetRole_Admin  PharmaSheetRole = "ADMIN"
	PharmaSheetRole_Editor PharmaSheetRole = "EDITOR"
	PharmaSheetRole_Viewer PharmaSheetRole = "VIEWER"
)

var PharmaSheetRoleAllValues = []PharmaSheetRole{
	PharmaSheetRole_Admin,
	PharmaSheetRole_Editor,
	PharmaSheetRole_Viewer,
}

func (e *PharmaSheetRole) Scan(value interface{}) error {
	var enumValue string
	switch val := value.(type) {
	case string:
		enumValue = val
	case []byte:
		enumValue = string(val)
	default:
		return errors.New("jet: Invalid scan value for AllTypesEnum enum. Enum value has to be of type string or []byte")
	}

	switch enumValue {
	case "ADMIN":
		*e = PharmaSheetRole_Admin
	case "EDITOR":
		*e = PharmaSheetRole_Editor
	case "VIEWER":
		*e = PharmaSheetRole_Viewer
	default:
		return errors.New("jet: Invalid scan value '" + enumValue + "' for PharmaSheetRole enum")
	}

	return nil
}

func (e PharmaSheetRole) String() string {
	return string(e)
}
