package migrate

import (
	"bufio"
	"io"
	"os"
	"strings"
)

type (
	fileIdentifier string

	file struct {
		dependencies []fileIdentifier
		sql          string
	}
)

func parseFile(path string) (file, error) {
	f, err := os.Open(path)
	if err != nil {
		return file{}, err
	}
	defer f.Close()

	return parseFileContent(f)
}

func parseFileContent(reader io.Reader) (file, error) {
	var file file

	buf := bufio.NewReader(reader)
	for {
		c, err := buf.Peek(1)
		if err != nil {
			return file, err
		}

		if c[0] != '#' {
			// No (more) dependencies, treat the rest as sql
			break
		}

		_, err = buf.ReadByte()
		if err != nil {
			return file, err
		}

		dep, err := buf.ReadString('\n')
		if err != nil {
			return file, err
		}

		file.dependencies = append(file.dependencies, fileIdentifier(strings.TrimSpace(dep)))
	}

	sql, err := io.ReadAll(buf)
	file.sql = string(sql)

	return file, err
}
