package search

import "io"

type RuneWriter interface {
	WriteRune(r rune) (int, error)
}

type Writer interface {
	io.StringWriter
	RuneWriter
}
