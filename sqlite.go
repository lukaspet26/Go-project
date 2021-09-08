/*
	SQLite client
	(c) Copyright David Thorpe 2017
	All Rights Reserved

	For Licensing and Usage information, please see LICENSE file
*/

package sqlite

import (
	"github.com/djthorpe/gopi"
)

/////////////////////////////////////////////////////////////////////
// TYPES & INTERFACES

type Type uint
type Flag uint

type Client interface {
	gopi.Driver

	// Reflect on data structure of a variable to return the rows we expect
	Reflect(v interface{}) ([]Column, error)
	PrimaryKey([]Column) (Key, error)
	//Unique([]Column) ([]Key, error)
	//Index([]Column) ([]Key, error)

	// Perform operation and return an error
	Do(Statement) error
}

type Column interface {
	Name() string
	Identifier() string // Either the name or custom identifier
	Type() Type
	Flag(Flag) bool
	Value(Flag) string
}

type Key interface{}

type Statement interface {
	// CREATE TABLE parameters
	Schema(string) Statement
	IfNotExists() Statement
	Temporary() Statement
	WithoutRowID() Statement

	// Return SQL string for the statement
	SQL() string
}

/////////////////////////////////////////////////////////////////////
// CONSTANTS

// These are the types we store 'natively' in SQLite
// in reality, they are converted from the basic types
// that SQLite stores
const (
	TYPE_NONE Type = iota
	TYPE_TEXT
	TYPE_INT
	TYPE_UINT
	TYPE_BOOL
	TYPE_FLOAT
	TYPE_BLOB
	TYPE_TIME
	TYPE_MAX
)

// These are various flags we use to modify when
// a table is created
const (
	FLAG_NONE     Flag = 0
	FLAG_NOT_NULL Flag = (1 << iota)
	FLAG_PRIMARY_KEY
	FLAG_UNIQUE_KEY
	FLAG_INDEX_KEY
	FLAG_NAME
	FLAG_TYPE
	FLAG_MAX = FLAG_TYPE
)

/////////////////////////////////////////////////////////////////////
// STRINGIFY

func (t Type) String() string {
	switch t {
	case TYPE_NONE:
		return "TYPE_NONE"
	case TYPE_TEXT:
		return "TYPE_TEXT"
	case TYPE_INT:
		return "TYPE_INT"
	case TYPE_UINT:
		return "TYPE_UINT"
	case TYPE_BOOL:
		return "TYPE_BOOL"
	case TYPE_FLOAT:
		return "TYPE_FLOAT"
	case TYPE_BLOB:
		return "TYPE_BLOB"
	case TYPE_TIME:
		return "TYPE_TIME"
	default:
		return "[?? Invalid Type value]"
	}
}

func (f Flag) String() string {
	switch f {
	case FLAG_NOT_NULL:
		return "FLAG_NOT_NULL"
	case FLAG_PRIMARY_KEY:
		return "FLAG_PRIMARY_KEY"
	case FLAG_UNIQUE_KEY:
		return "FLAG_UNIQUE_KEY"
	case FLAG_INDEX_KEY:
		return "FLAG_INDEX_KEY"
	case FLAG_NAME:
		return "FLAG_NAME"
	case FLAG_TYPE:
		return "FLAG_TYPE"
	default:
		return "[?? Invalid Flag value]"
	}
}
