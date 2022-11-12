package migrate

import (
	"strings"
	"testing"
)

const (
	testFileData1 = `single line command;`
	testFileData2 = `#dependency
			first command;
			second command;
			third
			command;`
	testFileData3 = `#dep1
#dep2`
)

func expectedDependencies(t *testing.T, file file, dependencies []string) {
	if len(dependencies) != len(file.dependencies) {
		t.Fatalf(
			"Erroneous number of dependencies. Expected %d got %d\n",
			len(dependencies),
			len(file.dependencies),
		)
	}

	for i := 0; i < len(dependencies); i++ {
		if dependencies[i] != string(file.dependencies[i]) {
			t.Fatalf(
				"Dependency missmatch.\nExpected: '%s'\nGot: '%s'\n",
				dependencies[i],
				file.dependencies[i],
			)
		}
	}
}

func TestFileParse(t *testing.T) {
	file, err := parseFileContent(strings.NewReader(testFileData1))
	if err != nil {
		t.Fatal(err)
	}

	expectedDependencies(t, file, nil)

	if file.sql != testFileData1 {
		t.Fatalf("Erroneous sql for file: %s", file.sql)
	}
}

func TestFileParseWithDependency(t *testing.T) {
	file, err := parseFileContent(strings.NewReader(testFileData2))
	if err != nil {
		t.Fatal(err)
	}

	expect := []string{
		"dependency",
	}

	expectedDependencies(t, file, expect)
}
