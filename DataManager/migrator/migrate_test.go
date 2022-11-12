package migrate

import (
	"database/sql"
	"fmt"
	"path"
	"runtime"
	"strings"
	"testing"
)

const (
	fA = "Hello World!"
	fB = `#filea
		CodeB`
	fC = `#filed
#fileb
		CodeC`
	fD = "filed"
)

type mockExecutor struct {
	t *testing.T
}

func (me mockExecutor) Exec(query string, values ...any) (sql.Result, error) {
	me.t.Logf("Executing query: '%s'\nwith values: %s", query, fmt.Sprint(values...))

	return nil, nil
}

func (me mockExecutor) Query(query string, values ...any) (*sql.Rows, error) {
	return nil, nil
}

func (me mockExecutor) QueryRow(query string, values ...any) *sql.Row {
	return nil
}

func errCaller(t *testing.T, err error) {
	_, fn, line, _ := runtime.Caller(2)
	fn = path.Base(fn)
	t.Fatalf("\n\t%s:%d:\n\t\t\t%v\n", fn, line, err)
}

func fatalError(t *testing.T, err error) {
	if err != nil {
		errCaller(t, err)
	}
}

func TestMigratorExecute(t *testing.T) {
	filea, err := parseFileContent(strings.NewReader(fA))
	fatalError(t, err)

	fileb, err := parseFileContent(strings.NewReader(fB))
	fatalError(t, err)

	filec, err := parseFileContent(strings.NewReader(fC))
	fatalError(t, err)

	filed, err := parseFileContent(strings.NewReader(fD))
	fatalError(t, err)

	var mig = Migrator{
		files: files{
			"filea": filea,
			"fileb": fileb,
			"filec": filec,
			"filed": filed,
		},
	}

	mock := mockExecutor{t}

	err = mig.executeFile(mock, "fileb")
	fatalError(t, err)
	err = mig.executeFile(mock, "filec")
	fatalError(t, err)
}

func checkForFile(t *testing.T, mig Migrator, fileID fileIdentifier) {
	if _, ok := mig.files[fileID]; !ok {
		errCaller(t, fmt.Errorf("missing file: %s", fileID))
	}
}

func TestMigratorFromDir(t *testing.T) {
	mig, err := FromDir("testdata")
	fatalError(t, err)

	checkForFile(t, mig, "posts_1")
	checkForFile(t, mig, "posts_2")
	checkForFile(t, mig, "users_1")

	fatalError(t, mig.EnqueueMigrations())

	mock := mockExecutor{t}

	for mig.Next() {
		fatalError(t, mig.Execute(mock))
	}
}

func TestMigratorReal(t *testing.T) {
	_, err := FromDir("sql")
	fatalError(t, err)
}
