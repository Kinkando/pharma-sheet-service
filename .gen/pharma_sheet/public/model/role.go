//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package model

import "errors"

type Role string

const (
	Role_Admin Role = "ADMIN"
	Role_User  Role = "USER"
)

var RoleAllValues = []Role{
	Role_Admin,
	Role_User,
}

func (e *Role) Scan(value interface{}) error {
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
		*e = Role_Admin
	case "USER":
		*e = Role_User
	default:
		return errors.New("jet: Invalid scan value '" + enumValue + "' for Role enum")
	}

	return nil
}

func (e Role) String() string {
	return string(e)
}
