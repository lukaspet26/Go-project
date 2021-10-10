package main

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"io"
	"net/http"
	"regexp"
	"strconv"

	// Packages
	router "github.com/djthorpe/go-server/pkg/httprouter"
	sqlite3 "github.com/djthorpe/go-sqlite/pkg/sqlite3"
	tokenizer "github.com/djthorpe/go-sqlite/pkg/tokenizer"

	// Namespace imports
	. "github.com/djthorpe/go-server"
	. "github.com/djthorpe/go-sqlite"
	. "github.com/djthorpe/go-sqlite/pkg/lang"

	// Some sort of hack
	_ "gopkg.in/yaml.v3"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type PingResponse struct {
	Version string       `json:"version"`
	Modules []string     `json:"modules"`
	Schemas []string     `json:"schemas"`
	Pool    PoolResponse `json:"pool"`
}

type PoolResponse struct {
	Cur int32 `json:"cur"`
	Max int32 `json:"max"`
}

type SchemaResponse struct {
	Schema   string                 `json:"schema"`
	Filename string                 `json:"filename,omitempty"`
	Memory   bool                   `json:"memory,omitempty"`
	Tables   []SchemaTableResponse  `json:"tables,omitempty"`
	Columns  []SchemaColumnResponse `json:"columns,omitempty"`
}

type SchemaTableResponse struct {
	Name    string                 `json:"name"`
	Schema  string                 `json:"schema"`
	Count   int64                  `json:"count"`
	Indexes []SchemaIndexResponse  `json:"indexes,omitempty"`
	Columns []SchemaColumnResponse `json:"columns,omitempty"`
}

type SchemaColumnResponse struct {
	Name     string `json:"name"`
	Table    string `json:"table"`
	Schema   string `json:"schema"`
	Type     string `json:"type"`
	Primary  bool   `json:"primary"`
	Nullable bool   `json:"nullable"`
}

type SchemaIndexResponse struct {
	Name    string   `json:"name"`
	Unique  bool     `json:"unique"`
	Columns []string `json:"columns"`
}

type SqlRequest struct {
	Sql string `json:"sql"`
}

type SqlResultResponse struct {
	Sql []string `json:"sql"`
}

type SyntaxResponse struct {
	Html     []template.HTML `json:"html,omitempty"`
	Complete bool            `json:"complete"`
}

///////////////////////////////////////////////////////////////////////////////
// ROUTES

var (
	reRoutePing     = regexp.MustCompile(`^/?$`)
	reRouteSchema   = regexp.MustCompile(`^/([a-zA-Z][a-zA-Z0-9_-]+)/?$`)
	reRouteSyntaxer = regexp.MustCompile(`^/-/syntax/?$`)
	reRouteQuery    = regexp.MustCompile(`^/-/q/?$`)
)

///////////////////////////////////////////////////////////////////////////////
// CONSTANTS

const (
	maxResultLimit = 1000
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (p *plugin) AddHandlers(ctx context.Context, provider Provider) error {
	// Add handler for ping
	if err := provider.AddHandlerFuncEx(ctx, reRoutePing, p.ServePing); err != nil {
		return err
	}

	// Add handler for schema
	if err := provider.AddHandlerFuncEx(ctx, reRouteSchema, p.ServeSchema); err != nil {
		return err
	}

	// Add handler for SQL syntax checker
	if err := provider.AddHandlerFuncEx(ctx, reRouteSyntaxer, p.ServeSyntaxer, http.MethodPost); err != nil {
		return err
	}

	// Add handler for queries
	if err := provider.AddHandlerFuncEx(ctx, reRouteQuery, p.ServeQuery, http.MethodPost); err != nil {
		return err
	}

	// Return success
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// HANDLERS

func (p *plugin) ServePing(w http.ResponseWriter, req *http.Request) {
	// Get a connection
	conn := p.Get(req.Context())
	if conn == nil {
		router.ServeError(w, http.StatusBadGateway, "No connection")
		return
	}
	defer p.Put(conn)

	// Populate response
	response := PingResponse{
		Schemas: []string{},
		Modules: []string{},
	}
	response.Version = sqlite3.Version()
	response.Schemas = append(response.Schemas, conn.Schemas()...)
	response.Modules = append(response.Modules, conn.Modules()...)
	response.Pool = PoolResponse{Cur: p.Cur(), Max: p.Max()}

	// Serve response
	router.ServeJSON(w, response, http.StatusOK, 2)
}

func (p *plugin) ServeSchema(w http.ResponseWriter, req *http.Request) {
	// Decode params, params[0] is the schema name
	params := router.RequestParams(req)

	// Get a connection
	conn := p.Get(req.Context())
	if conn == nil {
		router.ServeError(w, http.StatusBadGateway, "No connection")
		return
	}
	defer p.Put(conn)

	// Check for schema
	if stringSliceContainsElement(conn.Schemas(), params[0]) == false {
		router.ServeError(w, http.StatusNotFound, "Schema not found", strconv.Quote(params[0]))
		return
	}

	// Populate response
	response := SchemaResponse{
		Schema:   params[0],
		Filename: conn.Filename(params[0]),
		Tables:   []SchemaTableResponse{},
	}

	// Set memory flag
	if response.Filename == "" {
		response.Memory = true
	}

	// Populate tables
	for _, name := range conn.Tables(params[0]) {
		table := SchemaTableResponse{
			Name:    name,
			Schema:  params[0],
			Count:   conn.Count(params[0], name),
			Columns: []SchemaColumnResponse{},
			Indexes: []SchemaIndexResponse{},
		}
		for _, index := range conn.IndexesForTable(params[0], name) {
			table.Indexes = append(table.Indexes, SchemaIndexResponse{
				Name:    index.Name(),
				Unique:  index.Unique(),
				Columns: index.Columns(),
			})
		}
		for _, column := range conn.ColumnsForTable(params[0], name) {
			col := SchemaColumnResponse{
				Name:   column.Name(),
				Table:  name,
				Schema: params[0],
				Type:   column.Type(),
			}
			if column.Primary() != "" {
				col.Primary = true
			}
			table.Columns = append(table.Columns, col)
		}
		response.Tables = append(response.Tables, table)
	}

	// Serve response
	router.ServeJSON(w, response, http.StatusOK, 2)
}

func (p *plugin) ServeSyntaxer(w http.ResponseWriter, req *http.Request) {
	// Decode request
	query := SqlRequest{}
	if err := router.RequestBody(req, &query); err != nil {
		router.ServeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get a connection
	conn := p.Get(req.Context())
	if conn == nil {
		router.ServeError(w, http.StatusBadGateway, "No connection")
		return
	}
	defer p.Put(conn)

	// Tokenize input
	html, err := tokenize(query.Sql)
	if err != nil {
		router.ServeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Populate response
	response := SyntaxResponse{
		Html:     html,
		Complete: sqlite3.IsComplete(query.Sql),
	}

	// Serve response
	router.ServeJSON(w, response, http.StatusOK, 2)
}

func (p *plugin) ServeQuery(w http.ResponseWriter, req *http.Request) {
	// Decode request
	query := SqlRequest{}
	if err := router.RequestBody(req, &query); err != nil {
		router.ServeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get a connection
	conn := p.Get(req.Context())
	if conn == nil {
		router.ServeError(w, http.StatusBadGateway, "No connection")
		return
	}
	defer p.Put(conn)

	// Perform query
	response := make([]SqlResultResponse, 0)
	if err := conn.Do(req.Context(), SQLITE_TXN_DEFAULT, func(txn SQTransaction) error {
		_, err := txn.Query(Q(query.Sql))
		if err != nil {
			return err
		}
		// Return success
		return nil
	}); err != nil {
		router.ServeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Serve response
	router.ServeJSON(w, response, http.StatusOK, 2)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func stringSliceContainsElement(v []string, elem string) bool {
	for _, v := range v {
		if v == elem {
			return true
		}
	}
	return false
}

// tokenize will return an array of html spans, one for each token in the input
func tokenize(v string) ([]template.HTML, error) {
	result := []template.HTML{}

	// Iterate through the tokenizer
	t := tokenizer.NewTokenizer(v)
	for {
		token, err := t.Next()
		if token == nil || err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case tokenizer.KeywordToken:
			result = appendtoken(result, "keyword", t)
		case tokenizer.TypeToken:
			result = appendtoken(result, "type", t)
		case tokenizer.NameToken:
			result = appendtoken(result, "name", t)
		case tokenizer.ValueToken:
			result = appendtoken(result, "value", t)
		case tokenizer.PuncuationToken:
			result = appendtoken(result, "puncuation", t)
		case tokenizer.WhitespaceToken:
			result = appendtoken(result, "space", t)
		default:
			result = appendtoken(result, "", t)
		}
	}

	// Return success
	return result, nil
}

// Append token adds a html span to the result slice
func appendtoken(result []template.HTML, class string, value interface{}) []template.HTML {
	v := fmt.Sprint(value)
	if class != "" {
		return append(result, template.HTML("<span class="+strconv.Quote(class)+">"+html.EscapeString(v)+"</span>"))
	} else {
		return append(result, template.HTML("<span>"+html.EscapeString(v)+"</span>"))
	}
}
