package sqlite

import (
	"fmt"
	"strings"

	// Modules
	sqlite "github.com/djthorpe/go-sqlite"
	. "github.com/djthorpe/go-sqlite/pkg/lang"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	temporarySchema = "temp"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Schemas returns all attached schemas, except "temp"
func (this *connection) Schemas() []string {
	// Perform the query
	rs, err := this.Query(Q("PRAGMA database_list"))
	if err != nil {
		return nil
	}
	defer rs.Close()

	// Collate the results
	schemas := make([]string, 0, 1)
	for {
		row := rs.NextMap()
		if row == nil {
			break
		}
		schemas = append(schemas, row["name"].(string))
	}

	// Return success
	return schemas
}

// Filename returns the filename for a schema
func (this *connection) Filename(schema string) string {
	return this.conn.GetFilename(schema)
}

// Tables returns all known tables in main schema
func (this *connection) Tables() []string {
	return this.TablesEx("", false)
}

func (this *connection) TablesEx(schema string, temp bool) []string {
	// Create the query
	query := ""
	if temp {
		query = `
			SELECT name FROM 
   				(SELECT name,type FROM %ssqlite_master UNION ALL SELECT name,type FROM %ssqlite_temp_master)
			WHERE type=? AND name NOT LIKE 'sqlite_%%'
			ORDER BY name ASC
		`
	} else {
		query = `
			SELECT name FROM 
				%ssqlite_master 
			WHERE type=? AND name NOT LIKE 'sqlite_%%'
			ORDER BY name ASC -- %s
		`
	}

	// Append the schema
	if schema != "" {
		query = fmt.Sprintf(query, sqlite.QuoteIdentifier(schema)+".", sqlite.QuoteIdentifier(schema)+".")
	} else {
		query = fmt.Sprintf(query, "", "")
	}

	// Perform the query
	rows, err := this.Query(Q(query), "table")
	if err != nil {
		return nil
	}
	defer rows.Close()

	// Collate the results
	names := make([]string, 0, 10)
	for {
		values := rows.Next()
		if values == nil {
			break
		} else if len(values) != 1 {
			return nil
		} else {
			names = append(names, fmt.Sprint(values[0]))
		}
	}

	// Return success
	return names
}

func (this *connection) Columns(name string) []sqlite.SQColumn {
	return this.ColumnsEx(name, "")
}

func (this *connection) ColumnsEx(name, schema string) []sqlite.SQColumn {
	// Perform query
	rs, err := this.Query(Q("PRAGMA table_info(", N(name).WithSchema(schema), ")"))
	if err != nil {
		return nil
	}
	defer rs.Close()

	// Collate results, estimate up to 10 columns
	columns := make([]sqlite.SQColumn, 0, 10)
	for {
		row := rs.NextMap()
		if row == nil {
			break
		}
		col := N(row["name"].(string)).WithType(row["type"].(string))
		if row["notnull"].(int64) != 0 {
			col = col.NotNull()
		}
		columns = append(columns, col)
	}
	return columns
}

func (this *connection) Modules(prefix ...string) []string {
	// Perform query
	rs, err := this.Query(Q("PRAGMA module_list"))
	if err != nil {
		return nil
	}
	defer rs.Close()

	// Collate results
	var result []string
	for {
		row := rs.Next()
		if len(row) == 0 {
			break
		}
		module := row[0].(string)
		if moduleHasPrefix(module, prefix) {
			result = append(result, module)
		}
	}

	// Return nil if no matching results
	if len(result) == 0 {
		return nil
	} else {
		return result
	}
}

func (this *connection) Indexes(name string) []sqlite.SQIndexView {
	return this.IndexesEx(name, "")
}

func (this *connection) IndexesEx(name, schema string) []sqlite.SQIndexView {
	rs, err := this.Query(Q("PRAGMA index_list(", N(name).WithSchema(schema), ")"))
	if err != nil {
		return nil
	}
	defer rs.Close()

	// Collate results
	var result []sqlite.SQIndexView
	for {
		row := rs.NextMap()
		if row == nil {
			break
		}
		index := N(row["name"].(string)).
			WithSchema(schema).
			CreateIndex(name, this.indexColumns(row["name"].(string), schema)...)
		// Set temporary
		if schema == temporarySchema {
			index = index.WithTemporary()
		}
		// Set unique
		if row["unique"].(int64) != 0 || row["origin"].(string) == "u" || row["origin"].(string) == "pk" {
			index = index.WithUnique()
		}
		// Set whether a CREATE INDEX or AUTO INDEX
		if row["origin"].(string) != "c" {
			index = index.WithAuto()
		}

		// Append index
		result = append(result, index)
	}
	return result
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (this *connection) indexColumns(name, schema string) []string {
	// Query for index information
	rs, err := this.Query(Q("PRAGMA index_info(", N(name).WithSchema(schema), ")"))
	if err != nil {
		return nil
	}
	defer rs.Close()

	// Collate results
	var result []string
	for {
		row := rs.NextMap()
		if row == nil {
			break
		}
		if col, ok := row["name"].(string); ok {
			result = append(result, col)
		} else {
			result = append(result, fmt.Sprint("cid=", row["cid"]))
		}
	}
	return result
}

func moduleHasPrefix(module string, prefix []string) bool {
	if len(prefix) == 0 {
		return true
	}
	for _, prefix := range prefix {
		if strings.HasPrefix(module, prefix) {
			return true
		}
	}
	return false
}
