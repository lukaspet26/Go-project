package sqlite3

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	// Modules
	sqlite3 "github.com/djthorpe/go-sqlite/sys/sqlite3"
	multierror "github.com/hashicorp/go-multierror"

	// Namespace Imports
	. "github.com/djthorpe/go-errors"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// PoolConfig is the starting configuration for a pool
type PoolConfig struct {
	Max     int64             `yaml:"max"` // The maximum number of connections in the pool
	Schemas map[string]string `yaml:"db"`  // Schema names mapped onto path for database file
	Flags   sqlite3.OpenFlags // Flags for opening connections
}

// Pool is a connection pool object
type Pool struct {
	sync.Mutex
	sync.WaitGroup
	sync.Pool
	PoolConfig
	errs   chan<- error
	ctx    context.Context
	cancel context.CancelFunc
	n      int64
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	reSchemaName      = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_-]+$")
	defaultPoolConfig = PoolConfig{
		Max:     5,
		Schemas: map[string]string{defaultSchema: defaultMemory},
		Flags:   sqlite3.DefaultFlags | sqlite3.SQLITE_OPEN_SHAREDCACHE,
	}
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewPool returns a new default pool with a shared cache and maxiumum pool
// size of 5 connections. Pass a channel to receive errors, or nil to ignore
func NewPool(errs chan<- error) (*Pool, error) {
	return OpenPool(defaultPoolConfig, errs)
}

// OpenPool returns a new pool with the specified configuration
func OpenPool(config PoolConfig, errs chan<- error) (*Pool, error) {
	p := new(Pool)
	p.PoolConfig = config
	p.Pool = sync.Pool{New: p.new}
	p.Max = int64Max(p.Max, 0)
	p.errs = errs
	p.ctx, p.cancel = context.WithCancel(context.Background())
	return p, nil
}

// Close waits for all connections to be released and then
// releases resources
func (p *Pool) Close() error {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()

	// Send cancel signal to all workers and wait for them to exit
	p.cancel()
	p.Wait()

	// Return success
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (p *Pool) String() string {
	str := "<pool"
	str += fmt.Sprint(" cur=", atomic.AddInt64(&p.n, 0))
	str += fmt.Sprint(" max=", p.Max)
	for schema := range p.Schemas {
		str += fmt.Sprintf(" <schema %s=%q>", strings.TrimSpace(schema), p.pathForSchema(schema))
	}
	return str + ">"
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// SetMax allowed connections released from pool. Note this does not change
// the maximum instantly, it will settle to this value over time. Set as value
// zero to disable opening new connections
func (p *Pool) SetMax(n int64) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	p.Max = n
}

// Cur returns the current number of used connections
func (p *Pool) Cur() int64 {
	return atomic.AddInt64(&p.n, 0)
}

// Get a connection from the pool, and return it to the pool when the context
// is cancelled or it is put back using the Put method. If there are no
// connections available, nil is returned.
func (p *Pool) Get(ctx context.Context) *Conn {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()

	// Return error if maximum number of connections has been reached
	if atomic.AddInt64(&p.n, 0) >= int64(p.Max) {
		p.err(ErrChannelBlocked.Withf("Maximum number of connections (%d) reached", p.Cur()))
		return nil
	}

	// Get a connection from the pool, add one to counter
	conn := p.Pool.Get().(*Conn)
	if conn == nil {
		return nil
	}
	if conn.c != nil {
		panic("Expected conn.c to be nil")
	}
	conn.c = make(chan struct{})
	atomic.AddInt64(&p.n, 1)

	// Release the connection in the background
	p.WaitGroup.Add(1)
	go func() {
		defer p.WaitGroup.Done()
		select {
		case <-ctx.Done():
			p.put(conn)
		case <-conn.c:
			p.put(conn)
		case <-p.ctx.Done():
			p.put(conn)
		}
	}()

	// Return the connection
	return conn
}

// Return connection to the pool
func (p *Pool) Put(conn *Conn) {
	if conn != nil {
		conn.c <- struct{}{}
	}
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// Create a new connection and attach databases, returns error if unable to
// complete operation
func (p *Pool) new() interface{} {
	// Open connection to main schema, which is required
	defaultPath := p.pathForSchema(defaultSchema)
	if defaultPath == "" {
		p.err(ErrNotFound.Withf("No default schema %q found", defaultSchema))
		return nil
	}
	conn, err := OpenPath(defaultPath, p.Flags)
	if err != nil {
		p.err(err)
		return nil
	}

	// Attach additional databases
	var result error
	for schema := range p.Schemas {
		schema = strings.TrimSpace(schema)
		path := p.pathForSchema(schema)
		if path == defaultPath {
			continue
		}
		if path == "" {
			result = multierror.Append(result, ErrBadParameter.Withf("Schema %q", schema))
		} else if err := conn.Attach(schema, path); err != nil {
			result = multierror.Append(result, err)
		}
	}

	// Check for errors
	if result != nil {
		p.err(err)
		return nil
	}

	// Success
	return conn
}

func (p *Pool) put(conn *Conn) {
	if conn.c == nil {
		panic("Expected conn.c to be non-nil")
	}
	n := atomic.AddInt64(&p.n, -1)
	close(conn.c)
	conn.c = nil
	if n >= int64(p.Max) {
		p.Pool.Put(conn)
	} else {
		if err := conn.Close(); err != nil {
			p.err(err)
		}
	}
}

// pathForSchema returns the path for the specified schema
// or an empty string if the schema name is not valid
func (p *Pool) pathForSchema(schema string) string {
	if schema == "" {
		return p.pathForSchema(defaultSchema)
	} else if !reSchemaName.MatchString(schema) {
		return ""
	} else if path, exists := p.Schemas[schema]; !exists {
		return ""
	} else {
		return path
	}
}

// err will pass an error to a channel unless channel is blocked
func (p *Pool) err(err error) {
	select {
	case p.errs <- err:
		return
	default:
		return
	}
}

// int64Max returns the maximum of two values
func int64Max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
