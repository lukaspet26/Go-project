# sqlite3 package

This package provides a high-level interface for [sqlite3](http://sqlite.org/)
including connection pooling, transaction and execution management.

This package is part of a wider project, `github.com/djthorpe/go-sqlite`.
Please see the [module documentation](https://github.com/djthorpe/go-sqlite/blob/master/README.md)
for more information.

## Building

This module does not include a full
copy of __sqlite__ as part of the build process, but expect a `pkgconfig`
file called `sqlite3.pc` to be present (and an existing set of header
files and libraries to be available to link against, of course).

In order to locate the correct installation of `sqlite3` use two environment variables:

  * `PKG_CONFIG_PATH` is used for locating `sqlite3.pc`
  * `DYLD_LIBRARY_PATH` is used for locating the dynamic library when testing and/or running

On Macintosh with homebrew, for example:

```bash
[bash] brew install sqlite3
[bash] git clone git@github.com:djthorpe/go-sqlite.git
[bash] cd go-sqlite
[bash] go mod tidy
[bash] SQLITE_LIB="/usr/local/opt/sqlite/lib"
[bash] PKG_CONFIG_PATH="${SQLITE_LIB}/pkgconfig" DYLD_LIBRARY_PATH="${SQLITE_LIB}" go test -v ./pkg/sqlite3
```

On Debian Linux you shouldn't need to locate the correct path to the sqlite3 library:

```bash
[bash] sudo apt install libsqlite3-dev
[bash] git clone git@github.com:djthorpe/go-sqlite.git
[bash] cd go-sqlite
[bash] go mod tidy
[bash] go test -v ./pkg/sqlite3
```

There are some examples in the `cmd` folder of the main repository on how to use
the package, and various pseudo examples in this document.

## Contributing & Distribution

Please do file feature requests and bugs [here](https://github.com/djthorpe/go-sqlite/issues).
The license is Apache 2 so feel free to redistribute. Redistributions in either source
code or binary form must reproduce the copyright notice, and please link back to this
repository for more information:

> Copyright (c) 2021, David Thorpe, All rights reserved.

## Overview

The package includes:

  * A Connection __Pool__ for managing connections to sqlite3 databases;
  * A __Connection__ for executing queries;
  * An __Auth__ interface for managing authentication and authorization;
  * A __Cache__ for managing prepared statements and profiling for slow
    queries.

It's possible to create custom functions (both in a scalar and aggregate context)
and use perform streaming read and write operations on large binary (BLOB) objects.

In order to create a connection pool, you can create a default pool using the `NewPool`
method:

```go
package main

import (
    sqlite "github.com/djthorpe/go-sqlite/pkg/sqlite3"
)

func main() {
	pool, err := sqlite.NewPool(path, nil)
	if err != nil {
        panic(err)
	}
	defer pool.Close()

    // Onbtain a connection from pool, put back when done
    conn := pool.Get(context.Background())
    defer pool.Put(conn)

    // Enumerate the tables in the database
    tables := conn.Tables()

    // ...
}
```

In this example, a database is opened and the `Get` method obtains a connection
to the databaseand `Put` will return it to the pool. The `Tables` method enumerates 
the tables in the database.

## Connection Pool

TODO

## Transactions and Queries

TODO

## Custom Types

TODO

## Custom Functions

TODO

## Authentication and Authorization

TODO

## Pool Status

TODO

## Reading and Writing Large Objects

## Backup

