package sqlite

import (
	"fmt"

	// Modules
	sqlite "github.com/djthorpe/go-sqlite"
)

func (this *connection) Schemas() []string {
	// Perform the query
	rs, err := this.Query(this.Q("PRAGMA database_list"))
	if err != nil {
		return nil
	}
	defer rs.Close()

	// Collate the results
	schemas := make([]string, 0, 1)
	for {
		if row := rs.NextMap(); row == nil {
			break
		} else {
			schemas = append(schemas, row["name"].(string))
		}
	}

	// Return success
	return schemas
}

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
		query = fmt.Sprintf(query, QuoteIdentifier(schema)+".", QuoteIdentifier(schema)+".")
	} else {
		query = fmt.Sprintf(query, "", "")
	}

	// Perform the query
	rows, err := this.Query(this.Q(query), "table")
	if err != nil {
		return nil
	}
	defer rows.Close()

	// Collate the results
	names := make([]string, 0, 10)
	for {
		values := rows.NextArray()
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
	query := "table_info(" + QuoteIdentifier(name) + ")"
	if schema != "" {
		query = "PRAGMA " + QuoteIdentifier(schema) + "." + query
	} else {
		query = "PRAGMA " + query
	}
	rs, err := this.Query(this.Q(query))
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer rs.Close()

	// Collate results, estimate up to 10 columns
	columns := make([]sqlite.SQColumn, 0, 10)
	for {
		row := rs.NextMap()
		if row == nil {
			break
		} else {
			columns = append(columns, &column{
				name:     row["name"].(string),
				decltype: row["type"].(string),
				nullable: row["notnull"].(int64) == 0,
			})
		}
	}
	return columns
}
