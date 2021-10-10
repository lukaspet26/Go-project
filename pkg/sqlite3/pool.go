package sqlite3

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	// Modules
	sqlite3 "github.com/djthorpe/go-sqlite/sys/sqlite3"
	multierror "github.com/hashicorp/go-multierror"

	// Namespace Imports
	. "github.com/djthorpe/go-errors"
	. "github.com/djthorpe/go-sqlite"
	. "github.com/djthorpe/go-sqlite/pkg/lang"
	. "github.com/djthorpe/go-sqlite/pkg/quote"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// PoolConfig is the starting configuration for a pool
type PoolConfig struct {
	Max     int32             `yaml:"max"`       // The maximum number of connections in the pool
	Schemas map[string]string `yaml:"databases"` // Schema names mapped onto path for database file
	Trace   bool              `yaml:"trace"`     // Profiling for statements
	Create  bool              `yaml:"create"`    // When false, do not allow creation of new file-based databases
	Auth    SQAuth            // Authentication and Authorization interface
	Flags   sqlite3.OpenFlags // Flags for opening connections
}

// Pool is a connection pool object
type Pool struct {
	sync.WaitGroup
	sync.Pool
	PoolConfig
	PoolCache

	errs   chan<- error
	ctx    context.Context
	cancel context.CancelFunc
	n      int32
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	reSchemaName      = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_-]+$")
	defaultPoolConfig = PoolConfig{
		Max:     5,
		Trace:   false,
		Create:  true,
		Schemas: map[string]string{defaultSchema: defaultMemory},
		Flags:   sqlite3.SQLITE_OPEN_CREATE | sqlite3.SQLITE_OPEN_READWRITE | sqlite3.SQLITE_OPEN_SHAREDCACHE,
	}
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewPool returns a new default pool with a shared cache and maxiumum pool
// size of 5 connections. If filename is not empty, this database is opened
// or else memory is used. Pass a channel to receive errors, or nil to ignore
func NewPool(path string, errs chan<- error) (*Pool, error) {
	cfg := defaultPoolConfig
	if path != "" {
		cfg.Schemas = map[string]string{defaultSchema: path}
	}
	return OpenPool(cfg, errs)
}

// OpenPool returns a new pool with the specified configuration
func OpenPool(config PoolConfig, errs chan<- error) (*Pool, error) {
	p := new(Pool)

	// Set config.Max to default if zero, or minimum of 1
	// connection
	if config.Max == 0 {
		config.Max = defaultPoolConfig.Max
	} else {
		config.Max = maxInt32(config.Max, 1)
	}

	// Set default flags if not set
	if config.Flags == 0 {
		config.Flags = defaultPoolConfig.Flags
	}

	// Update create flag
	if config.Create {
		config.Flags |= sqlite3.SQLITE_OPEN_CREATE
	} else {
		config.Flags &^= sqlite3.SQLITE_OPEN_CREATE
	}

	// Set up pool
	p.PoolConfig = config
	p.Pool = sync.Pool{New: func() interface{} {
		if conn, errs := p.new(); errs != nil {
			p.err(errs)
			return nil
		} else {
			return conn
		}
	}}
	p.errs = errs
	p.ctx, p.cancel = context.WithCancel(context.Background())

	// Create a single connection and put in the pool
	if conn, errs := p.new(); errs != nil {
		return nil, errs
	} else {
		p.Pool.Put(conn)
	}

	// Return success
	return p, nil
}

// Close waits for all connections to be released and then
// releases resources
func (p *Pool) Close() error {
	// Set max to 0 to prevent new connections, send cancel signal to all workers
	// and wait for them to exit
	p.SetMax(0)
	p.cancel()
	p.Wait()

	// Return success
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (p *Pool) String() string {
	str := "<pool"
	str += fmt.Sprintf(" ver=%q", Version())
	str += fmt.Sprint(" cur=", p.Cur())
	str += fmt.Sprint(" max=", p.Max())
	str += fmt.Sprint(" flags=", p.Flags)
	for schema := range p.Schemas {
		str += fmt.Sprintf(" <schema %s=%q>", strings.TrimSpace(schema), p.pathForSchema(schema))
	}
	return str + ">"
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Max returns the maximum number of connections allowed
func (p *Pool) Max() int32 {
	return atomic.LoadInt32(&p.PoolConfig.Max)
}

// SetMax allowed connections released from pool. Note this does not change
// the maximum instantly, it will settle to this value over time. Set as value
// zero to disable opening new connections
func (p *Pool) SetMax(n int32) {
	atomic.StoreInt32(&p.PoolConfig.Max, maxInt32(n, 0))
}

// Cur returns the current number of used connections
func (p *Pool) Cur() int32 {
	return atomic.LoadInt32(&p.n)
}

// Get a connection from the pool, and return it to the pool when the context
// is cancelled or it is put back using the Put method. If there are no
// connections available, nil is returned.
func (p *Pool) Get(ctx context.Context) SQConnection {
	// Return error if maximum number of connections has been reached
	if p.Cur() >= p.Max() {
		p.err(ErrChannelBlocked.Withf("Maximum number of connections (%d) reached", p.Max()))
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
	atomic.AddInt32(&p.n, 1)
	conn.c = make(chan struct{})

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
func (p *Pool) Put(conn SQConnection) {
	if conn, ok := conn.(*Conn); ok {
		conn.c <- struct{}{}
	} else {
		panic(ErrBadParameter.With("Put"))
	}
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// Create a new connection and attach databases, returns error if unable to
// complete operation
func (p *Pool) new() (*Conn, error) {
	// Open connection to main schema, which is required
	defaultPath := p.pathForSchema(defaultSchema)
	if defaultPath == "" {
		return nil, ErrNotFound.Withf("No default schema %q found", defaultSchema)
	}

	// Always allow memory databases to be created and read/write
	flags := p.Flags
	if defaultPath == defaultMemory {
		flags |= (sqlite3.SQLITE_OPEN_CREATE | sqlite3.SQLITE_OPEN_READWRITE)
	}

	// Perform the open
	conn, err := OpenPath(defaultPath, flags)
	if err != nil {
		return nil, err
	}

	// Set trace
	if p.PoolConfig.Trace {
		conn.SetTraceHook(func(_ sqlite3.TraceType, a, b unsafe.Pointer) int {
			p.trace(conn, (*sqlite3.Statement)(a), *(*int64)(b))
			return 0
		}, sqlite3.SQLITE_TRACE_PROFILE)
	}

	// Attach additional databases
	var result error
	for schema := range p.Schemas {
		schema = strings.TrimSpace(schema)
		path := p.pathForSchema(schema)
		if schema == defaultSchema {
			continue
		}
		if path == "" {
			result = multierror.Append(result, ErrBadParameter.Withf("Schema %q", schema))
		} else if err := p.attach(conn, schema, path); err != nil {
			result = multierror.Append(result, err)
		}
	}

	// Set auth
	if p.PoolConfig.Auth != nil {
		conn.SetAuthorizerHook(func(action sqlite3.SQAction, args [4]string) sqlite3.SQAuth {
			if err := p.auth(conn.ctx, action, args); err == nil {
				return sqlite3.SQLITE_ALLOW
			} else {
				p.err(err)
				return sqlite3.SQLITE_DENY
			}
		})
	}

	// Check for errors
	if result != nil {
		return nil, result
	}

	// Success
	return conn, nil
}

func (p *Pool) put(conn *Conn) {
	if conn.c == nil {
		panic("Expected conn.c to be non-nil")
	}

	// Close channel
	close(conn.c)
	conn.c = nil

	// Choose to put back into pool or close connection
	n := atomic.AddInt32(&p.n, -1)
	if n < p.Max() {
		p.Pool.Put(conn)
	} else if err := conn.Close(); err != nil {
		p.err(err)
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

// Attach database as schema. If path is empty then a new in-memory database
// is attached.
func (p *Pool) attach(conn *Conn, schema, path string) error {
	if schema == "" {
		return ErrBadParameter.Withf("%q", schema)
	}
	if path == "" {
		return p.attach(conn, schema, defaultMemory)
	}
	// Create a new database or return an error if it doesn't exist
	if path != defaultMemory {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := p.attachCreate(path); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return conn.Exec(Q("ATTACH DATABASE ", Quote(path), " AS ", QuoteIdentifier(schema)), nil)
}

// Create a database before attaching
func (p *Pool) attachCreate(path string) error {
	if p.PoolConfig.Flags&sqlite3.SQLITE_OPEN_CREATE == 0 {
		return ErrBadParameter.Withf("Database does not exist: %q", path)
	}
	// Open then close database before attaching
	if conn, err := sqlite3.OpenPath(path, p.PoolConfig.Flags, ""); err != nil {
		return err
	} else if err := conn.Close(); err != nil {
		return err
	} else {
		return nil
	}
}

// Trace
func (p *Pool) trace(c *Conn, s *sqlite3.Statement, ns int64) {
	fmt.Printf("TRACE %q => %v\n", s, time.Duration(ns)*time.Nanosecond)
}

// maxInt32 returns the maximum of two values
func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
