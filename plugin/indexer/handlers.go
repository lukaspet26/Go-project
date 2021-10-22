package main

import (
	"context"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"

	// Packages
	router "github.com/mutablelogic/go-server/pkg/httprouter"
	indexer "github.com/mutablelogic/go-sqlite/pkg/indexer"
	version "github.com/mutablelogic/go-sqlite/pkg/version"

	// Namespace imports
	. "github.com/mutablelogic/go-server"
	. "github.com/mutablelogic/go-sqlite"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type PingResponse struct {
	Version map[string]string `json:"version"`
	Indexes []IndexResponse   `json:"indexes"`
}

type IndexResponse struct {
	Name    string      `json:"name"`
	Path    string      `json:"path,omitempty"`
	Count   int64       `json:"count,omitempty"`
	Modtime interface{} `json:"reindexed,omitempty"`
	Status  string      `json:"status,omitempty"`
}

type QueryRequest struct {
	Query   string `json:"q"`       // The query string
	Offset  uint   `json:"offset"`  // Offset within the result set
	Limit   uint   `json:"limit"`   // Limit the results
	Snippet bool   `json:"snippet"` // Whether to generate a snippet
}

type QueryResponse struct {
	Query   string           `json:"q"`
	Offset  uint             `json:"offset,omitempty"`
	Limit   uint             `json:"limit,omitempty"`
	Results []ResultResponse `json:"results"`
}

type ResultResponse struct {
	Id      int64        `json:"id"`
	Offset  int64        `json:"offset"`
	Rank    float64      `json:"rank"`
	Index   string       `json:"index"`
	Snippet string       `json:"snippet,omitempty"`
	File    FileResponse `json:"file"`
}

type FileResponse struct {
	Path     string    `json:"path"`
	Parent   string    `json:"parent"`
	Filename string    `json:"filename"`
	IsDir    bool      `json:"isdir,omitempty"`
	Ext      string    `json:"ext"`
	ModTime  time.Time `json:"modtime"`
	Size     int64     `json:"size"`
}

///////////////////////////////////////////////////////////////////////////////
// ROUTES

var (
	reRoutePing  = regexp.MustCompile(`^/?$`)
	reRouteQuery = regexp.MustCompile(`^/q/?$`)
)

///////////////////////////////////////////////////////////////////////////////
// CONSTANTS

const (
	maxResultLimit = 100
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func (p *plugin) AddHandlers(ctx context.Context, provider Provider) error {
	// Add handler for ping
	if err := provider.AddHandlerFuncEx(ctx, reRoutePing, p.ServePing); err != nil {
		return err
	}

	// Add handler for search
	if err := provider.AddHandlerFuncEx(ctx, reRouteQuery, p.ServeQuery); err != nil {
		return err
	}

	// Return success
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// HANDLERS

func (p *plugin) ServePing(w http.ResponseWriter, req *http.Request) {
	// Get a connection
	conn := p.pool.Get()
	if conn == nil {
		router.ServeError(w, http.StatusBadGateway, "No connection")
		return
	}
	defer p.pool.Put(conn)

	// Retrieve indexes with count of documents in each
	index, err := indexer.ListIndexWithCount(req.Context(), conn, p.store.Schema())
	if err != nil {
		router.ServeError(w, http.StatusBadGateway, err.Error())
		return
	}

	// Add known indexes to the response - these may not yet have any rows in the
	// database
	for _, idx := range p.index {
		name := idx.Name()
		if _, exists := index[name]; !exists {
			index[name] = 0
		}
	}

	// Populate response
	response := PingResponse{
		Version: version.Version(),
		Indexes: make([]IndexResponse, 0, len(index)),
	}

	// Add all indexes into the response, adding their modtime and
	// status
	for name, count := range index {
		response.Indexes = append(response.Indexes, IndexResponse{
			Name:    name,
			Count:   count,
			Path:    p.pathForIndex(name),
			Modtime: p.modtimeForIndex(name),
			Status:  p.statusForIndex(name),
		})
	}

	// Serve response
	router.ServeJSON(w, response, http.StatusOK, 2)
}

func (p *plugin) ServeQuery(w http.ResponseWriter, req *http.Request) {
	// Get a connection
	conn := p.pool.Get()
	if conn == nil {
		router.ServeError(w, http.StatusBadGateway, "No connection")
		return
	}
	defer p.pool.Put(conn)

	// Decode the query
	var query QueryRequest
	if err := router.RequestQuery(req, &query); err != nil {
		router.ServeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check query, offset and limit
	if query.Limit == 0 {
		query.Limit = maxResultLimit
	} else {
		query.Limit = uintMin(query.Limit, maxResultLimit)
	}
	query.Query = strings.TrimSpace(query.Query)
	if query.Query == "" {
		router.ServeError(w, http.StatusBadRequest, "missing q parameter")
		return
	}

	// Make a response
	response := QueryResponse{
		Query:   query.Query,
		Offset:  query.Offset,
		Limit:   query.Limit,
		Results: make([]ResultResponse, 0, query.Limit),
	}

	// Perform the query and collate the results
	if err := conn.Do(req.Context(), 0, func(txn SQTransaction) error {
		q := indexer.Query(p.store.Schema(), query.Snippet).WithLimitOffset(query.Limit, query.Offset)
		r, err := txn.Query(q, query.Query)
		if err != nil {
			return err
		}
		n := int64(0)
		for {
			rows := r.Next(nil, nil, nil, nil, nil, nil, nil, nil, nil, reflect.TypeOf(time.Time{}))
			if rows == nil {
				return nil
			} else {
				n = n + 1
			}
			response.Results = append(response.Results, ResultResponse{
				Id:      rows[0].(int64),
				Offset:  n + int64(query.Offset) - 1,
				Rank:    rows[1].(float64),
				Snippet: rows[2].(string),
				Index:   rows[3].(string),
				File: FileResponse{
					Path:     rows[4].(string),
					Parent:   rows[5].(string),
					Filename: rows[6].(string),
					IsDir:    int64ToBool(rows[7].(int64)),
					Ext:      rows[8].(string),
					ModTime:  rows[9].(time.Time),
					Size:     rows[10].(int64),
				},
			})
		}
	}); err != nil {
		router.ServeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Serve response
	router.ServeJSON(w, response, http.StatusOK, 2)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (p *plugin) pathForIndex(name string) string {
	if idx, exists := p.index[name]; exists {
		return idx.Path()
	} else {
		return ""
	}
}

func (p *plugin) modtimeForIndex(name string) interface{} {
	if t, exists := p.modtime[name]; exists && t.IsZero() == false {
		return t
	} else {
		return nil
	}
}

func (p *plugin) statusForIndex(name string) string {
	if idx, exists := p.index[name]; !exists {
		return ""
	} else if idx.IsIndexing() {
		return "indexing"
	} else if t, exists := p.modtime[name]; exists && t.IsZero() == false {
		return "ready"
	} else {
		return "pending"
	}
}
