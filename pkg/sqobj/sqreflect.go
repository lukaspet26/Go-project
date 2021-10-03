package sqobj

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	// Modules
	marshaler "github.com/djthorpe/go-marshaler"
	multierror "github.com/hashicorp/go-multierror"

	// Import namespaces
	. "github.com/djthorpe/go-errors"
	. "github.com/djthorpe/go-sqlite"
	. "github.com/djthorpe/go-sqlite/pkg/lang"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type SQReflect struct {
	t      reflect.Type
	col    []*sqcolumn
	colmap map[string]*sqcolumn
	idxmap map[string]*sqindex
}

type sqcolumn struct {
	*marshaler.Field
	Col     SQColumn
	Primary bool
	Index   bool
	Unique  bool
	Foreign bool
	Auto    bool
}

type sqindex struct {
	name   string
	unique bool
	cols   []string
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	timeType = reflect.TypeOf(time.Time{})
	blobType = reflect.TypeOf([]byte{})
)

const (
	tagNotNull       = "NOT NULL,NOTNULL"
	tagPrimary       = "PRIMARY,PRIMARY KEY"
	tagAutoincrement = "AUTOINCREMENT,AUTO"
	tagUnique        = "UNIQUE,UNIQUE KEY"
	tagForeign       = "FOREIGN,FOREIGN KEY"
	tagIndex         = "INDEX,INDEX KEY"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Return a reflection object for the given struct or nil if the argument is
// not a pointer to a struct or has no fields which are exported
func NewReflect(proto interface{}) (*SQReflect, error) {
	r := new(SQReflect)
	r.colmap = make(map[string]*sqcolumn)
	r.idxmap = make(map[string]*sqindex)

	// Set type - must be a struct
	if v := ValueOf(proto); !v.IsValid() {
		return nil, ErrBadParameter.Withf("%T", proto)
	} else {
		r.t = v.Type()
	}

	// Reflect fields
	fields := marshaler.NewEncoder(TagName).Reflect(proto)
	if len(fields) == 0 {
		return nil, ErrBadParameter.Withf("%T", proto)
	}

	// Set columns
	var result error
	for _, field := range fields {
		// Check for duplicate column name
		if _, exists := r.colmap[field.Name]; exists {
			result = multierror.Append(result, ErrDuplicateEntry.With(field.Name))
			continue
		}
		// Create column
		if col := newColumnFor(field); col == nil {
			result = multierror.Append(result, ErrInternalAppError.With(field.Name))
		} else {
			r.col = append(r.col, col)
			r.colmap[field.Name] = col
		}
	}
	// Set indexes
	for _, field := range fields {
		for _, tag := range field.Tags {
			name, unique := parseTagIndexValue(tag)
			if name != "" {
				if index, exists := r.idxmap[name]; !exists {
					r.idxmap[name] = &sqindex{name, unique, []string{field.Name}}
				} else if index.unique != unique {
					result = multierror.Append(result, ErrInternalAppError.With(field.Name))
				} else {
					index.cols = append(index.cols, field.Name)
				}
			}
		}
	}

	// Return success
	return r, result
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (this *SQReflect) String() string {
	str := "<sqreflect"
	str += fmt.Sprintf(" type=%q", this.t)
	str += fmt.Sprintf(" columns=%v", this.col)
	str += fmt.Sprintf(" indexes=%v", this.idxmap)
	return str + ">"
}

func (this *sqcolumn) String() string {
	str := "<" + this.Field.Name
	str += fmt.Sprintf(" sql=%q", this.Col)
	if this.Primary {
		str += " primary"
	}
	if this.Auto {
		str += " auto"
	}
	if this.Unique {
		str += " unique"
	}
	if this.Index {
		str += " index"
	}
	if this.Foreign {
		str += " foreign"
	}
	return str + ">"
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Return a column definition for a given column name
func (this *SQReflect) Column(name string) SQColumn {
	if col, exists := this.colmap[name]; !exists {
		return nil
	} else {
		return col.Col
	}
}

// Return column definitions
func (this *SQReflect) Columns() []SQColumn {
	result := make([]SQColumn, len(this.col))
	for i, col := range this.col {
		result[i] = col.Col
	}
	return result
}

// Return an index definition for a given index name and source table
func (this *SQReflect) Index(source SQSource, name string) SQIndexView {
	index, exists := this.idxmap[name]
	if !exists || source == nil || source.Name() == "" {
		return nil
	}
	st := N(source.Name()+"_"+name).WithSchema(source.Schema()).CreateIndex(source.Name(), index.cols...)
	if st == nil {
		return nil
	}
	if index.unique {
		st = st.WithUnique()
	}
	return st
}

// Return table and index definitions for a given source table
// adding IF NOT EXISTS to the table and indexes
func (this *SQReflect) Table(source SQSource, ifnotexists bool) []SQStatement {
	if source == nil || source.Name() == "" {
		return nil
	}
	result := make([]SQStatement, 1, len(this.idxmap)+1)

	// Create table statement
	table := source.CreateTable(this.Columns()...)
	if table == nil {
		return nil
	}
	if ifnotexists {
		table = table.IfNotExists()
	}
	for _, column := range this.col {
		if column.Unique {
			table = table.WithUnique(column.Field.Name)
		} else if column.Index {
			table = table.WithIndex(column.Field.Name)
		}
	}
	result[0] = table

	// Append index statements
	for _, index := range this.idxmap {
		st := this.Index(source, index.name)
		if st == nil {
			return nil
		}
		if ifnotexists {
			st = st.IfNotExists()
		}
		result = append(result, st)
	}

	// Return success
	return result
}

///////////////////////////////////////////////////////////////////////////////
// STATIC METHODS

// ValueOf returns a struct value or nil if not valid
func ValueOf(v interface{}) reflect.Value {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	return rv
}

// DeclType returns the declared column type for a given field
// uses TEXT by default. Accepts both scalar types and pointer types
func DeclType(t reflect.Type) string {
	// Convert pointer type to element type
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "INTEGER"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "INTEGER"
	case reflect.Float32, reflect.Float64:
		return "FLOAT"
	case reflect.Bool:
		return "INTEGER"
	case reflect.Slice:
		if t == blobType {
			return "BLOB"
		}
	case reflect.Struct:
		if t == timeType {
			return "TIMESTAMP"
		}
	}
	return "TEXT"
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// newColumnFor returns a new column for the given field or nil if there
// is some sort of error
func newColumnFor(f *marshaler.Field) *sqcolumn {
	this := new(sqcolumn)
	if f == nil {
		return nil
	}

	// Set field and column
	this.Field = f
	this.Col = C(f.Name).WithType(DeclType(f.Type))

	// If field value is not zero type, then set default=true
	if !f.Value.IsZero() && f.Value.CanInterface() {
		this.Col = this.Col.WithDefault(f.Value.Interface())
	}

	// Cycle through tags
	for _, tag := range f.Tags {
		tag = strings.TrimSpace(strings.ToUpper(tag))
		// If tag is BOOL, INTEGER, FLOAT, TEXT, BLOB, TIMESTAMP, then set column type
		if IsSupportedType(tag) {
			this.Col = this.Col.WithType(strings.ToUpper(tag))
			continue
		}
		// Check for other tags, ignore unrecognized tags
		switch {
		case isTag(tag, tagNotNull):
			this.Col = this.Col.NotNull()
		case isTag(tag, tagPrimary):
			this.Col = this.Col.WithPrimary().NotNull()
			this.Primary = true
		case isTag(tag, tagAutoincrement):
			this.Col = this.Col.WithAutoIncrement().WithPrimary().NotNull()
			this.Primary = true
			this.Auto = true
		case isTag(tag, tagUnique):
			this.Unique = true
		case isTag(tag, tagIndex):
			this.Index = true
		case isTag(tag, tagForeign):
			this.Foreign = true
		}
	}
	return this
}

// parseTagIndexValue returns name of index and whether the index is
// unique or not. Returns empty string if not recognized
func parseTagIndexValue(tag string) (string, bool) {
	tag_name := strings.SplitN(tag, ":", 2)
	if len(tag_name) == 2 {
		tag = strings.TrimSpace(strings.ToUpper(tag_name[0]))
		if isTag(tag, tagUnique) {
			return tag_name[1], true
		} else if isTag(tag, tagIndex) {
			return tag_name[1], false
		}
	}
	return "", false
}

func isTag(v string, keywords string) bool {
	for _, keyword := range strings.Split(keywords, ",") {
		if v == keyword {
			return true
		}
	}
	return false
}
