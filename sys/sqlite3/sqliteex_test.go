package sqlite3_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/djthorpe/go-sqlite/sys/sqlite3"
)

const (
	longRunningQuery = `WITH RECURSIVE r(i) AS (
		VALUES(0)
		UNION ALL
		SELECT i FROM r
		LIMIT ?
	  ) SELECT i FROM r WHERE i = 1;`
)

func Test_SQLiteEx_001(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	db, err := sqlite3.OpenPathEx(filepath.Join(tmpdir, "test.sqlite"), sqlite3.SQLITE_OPEN_CREATE, "")
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	for i := 0; i < 10; i++ {
		st, err := db.Prepare(fmt.Sprint("SELECT ", i))
		if err != nil {
			t.Error(err)
		}
		defer st.Close()
		r, err := st.Exec(0)
		if err != nil {
			t.Error(err)
		}
		t.Log(r)
		for {
			if row, err := r.Next(); err != nil {
				t.Error(err)
				break
			} else if row == nil {
				break
			} else {
				t.Log(row)
			}
		}
	}
}

func Test_SQLiteEx_002(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	db, err := sqlite3.OpenPathEx(filepath.Join(tmpdir, "test.sqlite"), sqlite3.SQLITE_OPEN_CREATE, "")
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	// Add progress handler with context timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := db.SetProgressHandler(1000, func() bool {
		t.Log("Long running query...")
		return ctx.Err() != nil
	}); err != nil {
		t.Error(err)
	}

	// Add busy handler with context timeout
	if err := db.SetBusyHandler(func(n int) bool {
		t.Log("Called busy handler with n=", n)
		return true
	}); err != nil {
		t.Error(err)
	}

	// Add auth handler
	if err := db.SetAuthorizerHook(func(action sqlite3.SQAction, args [4]string) sqlite3.SQAuth {
		t.Logf("Called auth handler with %v %q", action, args)
		return sqlite3.SQLITE_ALLOW
	}); err != nil {
		t.Error(err)
	}

	// Run long running query, expect interrupted error
	if st, err := db.Prepare(longRunningQuery); err != nil {
		t.Error(err)
	} else if r, err := st.Exec(0, 99999999); err != nil && err != sqlite3.SQLITE_INTERRUPT {
		t.Error("Error returned:", err)
	} else if r != nil {
		t.Log(r)
		for {
			row, err := r.Next()
			if err != nil {
				t.Error(err)
				break
			} else if row == nil {
				break
			}
			t.Log(row)
		}
	}
}

func Test_SQLiteEx_003(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	db, err := sqlite3.OpenPathEx(filepath.Join(tmpdir, "test.sqlite"), sqlite3.SQLITE_OPEN_CREATE, "")
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	// Run a PRAGMA operation
	if err := db.Exec("PRAGMA module_list; PRAGMA compile_options;", func(row, cols []string) bool {
		t.Logf("row=%q cols=%q", row, cols)
		return false
	}); err != nil {
		t.Error(err)
	}
}

func Test_SQLiteEx_004(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	db, err := sqlite3.OpenPathEx(filepath.Join(tmpdir, "test.sqlite"), sqlite3.SQLITE_OPEN_CREATE, "")
	if err != nil {
		t.Error(err)
	}
	defer db.Close()

	// Add commit and rollback hooks
	if err := db.SetCommitHook(func() bool {
		t.Log("Commit hook called")
		return true
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetRollbackHook(func() {
		t.Log("Rollback hook called")
	}); err != nil {
		t.Fatal(err)
	}

	// Do a begin and commit
	if err := db.Begin(sqlite3.SQLITE_TXN_DEFAULT); err != nil {
		t.Error(err)
	} else if err := db.Commit(); err != nil {
		t.Error(err)
	}

	// Do a begin and rollback
	if err := db.Begin(sqlite3.SQLITE_TXN_DEFAULT); err != nil {
		t.Error(err)
	} else if err := db.Rollback(); err != nil {
		t.Error(err)
	}
}
