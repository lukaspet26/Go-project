package sqlite_test

import (
	"strconv"
	"testing"
	"time"

	// Frameworks
	gopi "github.com/djthorpe/gopi"
	sq "github.com/djthorpe/sqlite"
	sqlite "github.com/djthorpe/sqlite/sys/sqlite"
)

func Test_001(t *testing.T) {
	t.Log("Test_001")
}
func Test_002(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else if err := driver.Close(); err != nil {
		t.Error(err)
	}
}

func Test_003(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		defer driver.Close()
		if sqlite, ok := driver.(sq.Connection); !ok {
			t.Error("Cannot cast connection object")
			_ = driver.(sq.Connection)
		} else {
			t.Log(sqlite)
		}
	}
}

func Test_004(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()
		if tables := driver_.Tables(); tables == nil {
			t.Error("Expected Tables to return empty slice")
		}
	}
}

func Test_005(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()
		if _, err := driver_.DoOnce("CREATE TABLE test (a,b)"); err != nil {
			t.Error("Expected Tables to return empty slice")
		} else if tables := driver_.Tables(); tables == nil {
			t.Error("Expected Tables to return empty slice")
		} else if len(tables) != 1 {
			t.Error("Expected Tables to return a single value")
		} else if tables[0] != "test" {
			t.Errorf("Unexpected return value from Tables: %v", tables)
		}
	}
}

func Test_006(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()
		if _, err := driver_.DoOnce("CREATE TABLE test (a,b); CREATE TABLE test2 (c,d)"); err != nil {
			t.Error("Expected Tables to return empty slice")
		} else if tables := driver_.Tables(); tables == nil {
			t.Error("Expected Tables to return empty slice")
		} else if len(tables) != 2 {
			t.Error("Expected Tables to return a single value")
		} else if tables[0] != "test" {
			t.Errorf("Unexpected return value from Tables: %v", tables)
		} else if tables[1] != "test2" {
			t.Errorf("Unexpected return value from Tables: %v", tables)
		}
	}
}

func Test_007(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()
		if _, err := driver_.DoOnce("CREATE TABLE test (a,b)"); err != nil {
			t.Error("Expected Tables to return empty slice")
		} else if results, err := driver_.DoOnce("INSERT INTO test VALUES (1,2)"); err != nil {
			t.Error(err)
		} else if results.RowsAffected != 1 {
			t.Errorf("Unexpected RowsAffected value: %v", results)
		}
	}
}

func Test_008(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()
		if _, err := driver_.DoOnce("CREATE TABLE test (a,b)"); err != nil {
			t.Error("Expected Tables to return empty slice")
		} else if results, err := driver_.DoOnce("INSERT INTO test VALUES (1,2),(3,4)"); err != nil {
			t.Error(err)
		} else if results.RowsAffected != 2 {
			t.Errorf("Unexpected RowsAffected value: %v", results)
		}
	}
}

func Test_009(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()
		if _, err := driver_.DoOnce("CREATE TABLE test (a INTEGER,b INTEGER)"); err != nil {
			t.Error("Expected Tables to return empty slice")
		} else if _, err := driver_.DoOnce("INSERT INTO test VALUES (1,2),(3,4)"); err != nil {
			t.Error(err)
		} else if st, err := driver_.Prepare("SELECT a,b FROM test"); err != nil {
			t.Error(err)
		} else if rs, err := driver_.Query(st); err != nil {
			t.Error(err)
		} else {
			t.Log(rs)
			for {
				row := rs.Next()
				if row == nil {
					break
				}
				t.Log(row)
			}
		}
	}
}

func Test_010(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()
		if _, err := driver_.DoOnce("CREATE TABLE test (a timestamp)"); err != nil {
			t.Error("Expected Tables to return empty slice")
		} else if _, err := driver_.DoOnce("INSERT INTO test VALUES (?)", time.Now()); err != nil {
			t.Error(err)
		} else if st, err := driver_.Prepare("SELECT a FROM test"); err != nil {
			t.Error(err)
		} else if rs, err := driver_.Query(st); err != nil {
			t.Error(err)
		} else {
			for {
				row := rs.Next()
				if row == nil {
					break
				}
				t.Log(row[0].String())
				t.Log(row[0].Timestamp())
			}
		}
	}
}

func Test_011(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()
		if _, err := driver_.DoOnce("CREATE TABLE test (a integer,b integer)"); err != nil {
			t.Error("Expected Tables to return empty slice")
		} else if insert, err := driver_.Prepare("INSERT INTO test (a,b) VALUES (?,?)"); err != nil {
			t.Error(err)
		} else if _, err := driver_.DoOnce("INSERT INTO test (a,b) VALUES (0,12)"); err != nil {
			t.Error(err)
		} else if _, err := driver_.Do(insert, 1, 34); err != nil {
			t.Error(err)
		} else if _, err := driver_.DoOnce("INSERT INTO test (a,b) VALUES (2,NULL)"); err != nil {
			t.Error(err)
		} else if _, err := driver_.DoOnce("INSERT INTO test (a,b) VALUES (3,?)", nil); err != nil {
			t.Error(err)
		} else if rs, err := driver_.QueryOnce("SELECT a,b FROM test ORDER BY a"); err != nil {
			t.Error(err)
		} else {
			for i := 0; true; i++ {
				row := rs.Next()
				if row == nil {
					break
				}
				t.Log(sq.RowString(row))
				switch i {
				case 0:
					if len(row) != 2 || row[0].Int() != 0 || row[1].Int() != 12 {
						t.Errorf("Unexpected row[%v]: %v", i, sq.RowString(row))
					}
				case 1:
					if len(row) != 2 || row[0].Int() != 1 || row[1].Int() != 34 {
						t.Errorf("Unexpected row[%v]: %v", i, sq.RowString(row))
					}
				case 2:
					if len(row) != 2 || row[0].Int() != 2 || row[1].IsNull() == false {
						t.Errorf("Unexpected row[%v]: %v", i, sq.RowString(row))
					}
				case 3:
					if len(row) != 2 || row[0].Int() != 3 || row[1].IsNull() == false {
						t.Errorf("Unexpected row[%v]: %v", i, sq.RowString(row))
					}
				}
			}
		}
	}
}

func Test_012(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()
		if _, err := driver_.DoOnce("CREATE TABLE test (a blob)"); err != nil {
			t.Error("Expected Tables to return empty slice")
		} else if _, err := driver_.DoOnce("INSERT INTO test VALUES (?)", "hello"); err != nil {
			t.Error(err)
		} else if _, err := driver_.DoOnce("INSERT INTO test VALUES (?)", []byte{0, 1, 2, 3, 4}); err != nil {
			t.Error(err)
		} else if rs, err := driver_.QueryOnce("SELECT a FROM test ORDER BY a"); err != nil {
			t.Error(err)
		} else {
			t.Log(rs)
			for {
				row := rs.Next()
				if row == nil {
					break
				}
				t.Log(sq.RowString(row))
			}
		}
	}
}

func Test_013(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()

		tests := []struct {
			f     func() sq.CreateTable
			query string
		}{
			{func() sq.CreateTable { return driver_.NewCreateTable("test") }, "CREATE TABLE test ()"},
			{func() sq.CreateTable { return driver_.NewCreateTable("test").Schema("test") }, "CREATE TABLE test.test ()"},
			{func() sq.CreateTable { return driver_.NewCreateTable("test").Temporary() }, "CREATE TEMPORARY TABLE test ()"},
			{func() sq.CreateTable { return driver_.NewCreateTable("test").IfNotExists() }, "CREATE TABLE IF NOT EXISTS test ()"},
			{func() sq.CreateTable { return driver_.NewCreateTable("test").WithoutRowID() }, "CREATE TABLE test () WITHOUT ROWID"},
		}

		for i, test := range tests {
			if statement := test.f(); statement == nil {
				t.Errorf("Test %v: nil value returned", i)
			} else if statement.Query() != test.query {
				t.Errorf("Test %v: Expected %v, got %v", i, strconv.Quote(test.query), strconv.Quote(statement.Query()))
			}
		}
	}
}

func Test_014(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()

		if statement := driver_.NewCreateTable("test", driver_.NewColumn("a", "TEXT", false), driver_.NewColumn("b", "TEXT", true)); statement == nil {
			t.Error("Statement returned is nil")
		} else if statement.Query() != "CREATE TABLE test (a TEXT NOT NULL,b TEXT)" {
			t.Errorf("Unexpected value, %v", statement.Query())
		}
	}
}

func Test_015(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()

		if statement := driver_.NewCreateTable("test", driver_.NewColumn("a", "TEXT", false), driver_.NewColumn("b", "TEXT", true)); statement == nil {
			t.Error("Statement returned is nil")
		} else if statement.PrimaryKey("a").Query() != "CREATE TABLE test (a TEXT NOT NULL,b TEXT,PRIMARY KEY (a))" {
			t.Errorf("Unexpected value, %v", statement.Query())
		}
	}
}

func Test_016(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()

		if statement := driver_.NewCreateTable("test", driver_.NewColumn("a", "TEXT", false), driver_.NewColumn("b", "TEXT", true)); statement == nil {
			t.Error("Statement returned is nil")
		} else if statement.PrimaryKey("a").Unique("a", "b").Query() != "CREATE TABLE test (a TEXT NOT NULL,b TEXT,PRIMARY KEY (a),UNIQUE (a,b))" {
			t.Errorf("Unexpected value, %v", statement.Query())
		}

		if statement := driver_.NewCreateTable("test", driver_.NewColumn("a", "TEXT", false), driver_.NewColumn("b", "TEXT", true)); statement == nil {
			t.Error("Statement returned is nil")
		} else if statement.Unique("a").Unique("b").Query() != "CREATE TABLE test (a TEXT NOT NULL,b TEXT,UNIQUE (a),UNIQUE (b))" {
			t.Errorf("Unexpected value, %v", statement.Query())
		}

	}
}

func Test_017(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()

		if statement := driver_.NewCreateTable("test", driver_.NewColumn("a", "TEXT", false), driver_.NewColumn("b", "TEXT", true)); statement == nil {
			t.Error("Statement returned is nil")
		} else if _, err := driver_.DoOnce(statement.Query()); err != nil {
			t.Error(err)
		}

		if _, err := driver_.DoOnce(driver_.NewDropTable("test").Query()); err != nil {
			t.Error(err)
		}

		if statement := driver_.NewCreateTable("test", driver_.NewColumn("a", "TEXT", false), driver_.NewColumn("b", "TEXT", true)); statement == nil {
			t.Error("Statement returned is nil")
		} else if _, err := driver_.DoOnce(statement.PrimaryKey("b").Query()); err != nil {
			t.Error(err)
		}

	}
}

func Test_018(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()

		if statement := driver_.NewDropTable("test"); statement.Query() != "DROP TABLE test" {
			t.Error("Unexpected query:", statement.Query())
		} else if statement.IfExists(); statement.Query() != "DROP TABLE IF EXISTS test" {
			t.Error("Unexpected query:", statement.Query())
		} else if statement.Schema("test"); statement.Query() != "DROP TABLE IF EXISTS test.test" {
			t.Error("Unexpected query:", statement.Query())
		}
	}
}
func DISABLED_Test_019(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()

		if statement := driver_.NewInsert("test"); statement.Query() != "INSERT INTO test VALUES (?)" {
			t.Error("Unexpected query:", statement.Query())

		}
	}
}

func Test_Reflect_001(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()

		if columns, err := driver_.Reflect(struct{}{}); err != nil {
			t.Error(err)
		} else {
			t.Log(columns)
		}
	}
}

func Test_Reflect_002(t *testing.T) {
	if driver, err := gopi.Open(sqlite.Config{}, nil); err != nil {
		t.Error(err)
	} else {
		driver_ := driver.(sq.Connection)
		defer driver_.Close()

		if columns, err := driver_.Reflect(struct{ a int }{}); err != nil {
			t.Error(err)
		} else if len(columns) != 0 {
			t.Error("Expected zero returned columns")
		}

		if columns, err := driver_.Reflect(struct{ A int }{}); err != nil {
			t.Error(err)
		} else if len(columns) != 1 {
			t.Error("Expected one returned columns")
		} else {
			t.Log(columns)
		}

		if columns, err := driver_.Reflect(struct{ A, B int }{}); err != nil {
			t.Error(err)
		} else if len(columns) != 2 {
			t.Error("Expected two returned columns")
		} else {
			t.Log(columns)
		}

		if columns, err := driver_.Reflect(struct {
			A int `sql:"test"`
		}{}); err != nil {
			t.Error(err)
		} else if len(columns) != 1 {
			t.Error("Expected two returned columns", columns)
		} else if columns[0].Name() != "test" {
			t.Error("Expected column name 'test'", columns)
		} else {
			t.Log(columns)
		}

		if columns, err := driver_.Reflect(struct {
			A int `sql:",nullable"`
		}{}); err != nil {
			t.Error(err)
		} else if len(columns) != 1 {
			t.Error("Expected one returned columns", columns)
		} else if columns[0].Name() != "A" {
			t.Error("Expected column name 'A'", columns)
		} else if columns[0].Nullable() != true {
			t.Error("Expected column nullable", columns)
		} else {
			t.Log(columns)
		}

		if columns, err := driver_.Reflect(struct {
			A string `sql:"TEST WITH SPACES,nullable,bool"`
		}{}); err != nil {
			t.Error(err)
		} else if len(columns) != 1 {
			t.Error("Expected one returned column", columns)
		} else if columns[0].Name() != "TEST WITH SPACES" {
			t.Error("Expected column name 'TEST WITH SPACES'", columns)
		} else if columns[0].Nullable() != true {
			t.Error("Expected column nullable", columns)
		} else if columns[0].DeclType() != "BOOL" {
			t.Error("Expected column type BOOL", columns)
		} else {
			t.Log(columns)
		}

	}
}
