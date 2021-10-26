package sqobj

import (
	"fmt"
	"reflect"

	// Import Namespaces
	. "github.com/djthorpe/go-errors"
	. "github.com/mutablelogic/go-sqlite"
	. "github.com/mutablelogic/go-sqlite/pkg/lang"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type View struct {
	*SQReflect
	SQSource

	// Prepared statements and in-place parameters
	st SQSelect
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// MustRegisterView registers a SQObject view class, panics if an error
// occurs.
func MustRegisterView(name SQSource, proto interface{}, leftjoin bool, sources ...SQClass) *View {
	if cls, err := RegisterView(name, proto, leftjoin, sources...); err != nil {
		panic(err)
	} else {
		return cls
	}
}

// RegisterView registers a SQObject view class, returns the class and any errors
func RegisterView(name SQSource, proto interface{}, leftjoin bool, sources ...SQClass) (*View, error) {
	this := new(View)

	// Check name
	if name.Name() == "" {
		return nil, ErrBadParameter.With("source")
	} else {
		this.SQSource = name
	}

	// Do reflection
	if r, err := NewReflect(proto); err != nil {
		return nil, err
	} else {
		this.SQReflect = r
	}

	// At the moment we only support exactly two sources. Will fix this later!
	if len(sources) != 2 {
		return nil, ErrNotImplemented.With("currently only support joining two sources to create a view")
	}

	// Generate the view select statement
	j := this.join(sources[0].(*Class), sources[1].(*Class), leftjoin)
	if j == nil {
		return nil, ErrBadParameter.With("sources could not be joined")
	}
	// resolve columns from the classes
	to := this.to(sources[0].(*Class), sources[1].(*Class))
	if to == nil {
		return nil, ErrBadParameter.With("columns could not be resolved")
	}
	this.st = S(j).To(to...)

	// Return success
	return this, nil
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (this *View) String() string {
	str := "<sqview"
	str += fmt.Sprintf(" name=%q", this.Name())
	if schema := this.Schema(); schema != "" {
		str += fmt.Sprintf(" schema=%q", this.Schema())
	}
	str += fmt.Sprintf(" select=%q", this.st)
	str += " " + fmt.Sprint(this.SQReflect)
	return str + ">"
}

///////////////////////////////////////////////////////////////////////////////
// PROPERTIES

// Proto returns a prototype of the class
func (this *View) Proto() reflect.Value {
	return reflect.New(this.t)
}

// Select returns the select statement for the view
func (this *View) Select() SQSelect {
	return this.st
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Create creates a view. If
// the flag SQLITE_OPEN_OVERWRITE is set when creating the connection, then view
// is dropped and then re-created.
func (this *View) Create(txn SQTransaction, schema string) error {
	// If schema then set it
	if schema != "" {
		this.SQSource = this.SQSource.WithSchema(schema)
	}

	if txn.Flags().Is(SQLITE_OPEN_OVERWRITE) && hasElement(txn.Views(this.Schema()), this.Name()) {
		// Drop view
		if _, err := txn.Query(this.DropView()); err != nil {
			return err
		}
	}

	// Create view
	if _, err := txn.Query(this.View(this.SQSource, this.st, true)); err != nil {
		return err
	}

	// Return success
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// Return a join between two classes. JOIN or LEFT JOIN
func (this *View) join(l, r *Class, leftjoin bool) SQJoin {
	if l == nil || r == nil {
		return nil
	}

	// Find all join aliases which are in both classes
	aliases := make([]string, 0, len(l.joinmap))
	for k := range l.joinmap {
		if _, exists := r.joinmap[k]; exists {
			aliases = append(aliases, k)
		}
	}

	// If there is no intersection between the two tables, return nil
	if len(aliases) == 0 {
		return nil
	}

	// Return a join:
	//   this [LEFT] JOIN other ON this.alias = other.alias AND this.alias = other.alias
	// or if the column names are the same,
	//   this [LEFT} JOIN other USING (alias,alias)
	join := J(l.SQSource, r.SQSource)
	expr := make([]SQExpr, 0, len(aliases))
	using := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		lcol := l.joinmap[alias]
		rcol := r.joinmap[alias]
		if lcol.Name == rcol.Name {
			using = append(using, lcol.Name)
		}
		expr = append(expr, Q(N(lcol.Name), "=", N(rcol.Name)))
	}
	if len(using) == len(expr) {
		join = join.Using(using...)
		expr = nil
	}
	if leftjoin {
		join = join.LeftJoin(expr...)
	} else {
		join = join.Join(expr...)
	}

	// Return success
	return join
}

// Return a "to" select phrase for columns from classes
func (this *View) to(source ...*Class) []SQExpr {
	result := make([]SQExpr, 0, len(this.col))
	// Add the columns from the view
	for _, col := range this.col {
		var dest SQExpr
		for _, source := range source {
			if col := source.Column(col.Name); col != nil {
				dest = C(col.Name()).WithAlias(col.Name()).WithSchema(source.Name())
				break
			}
		}
		if dest == nil {
			// Column could not be resolved
			return nil
		}
		result = append(result, dest)
	}

	return result
}
