package image

import "io"

type Thumbnailer interface {
	Accepts(string) bool
	Resize(io.ReadSeeker, Format) (io.ReadSeekCloser, error)
}

type ErrNoThumbnailer string

func (e ErrNoThumbnailer) Error() string {
	return "No thumbnailer found for: " + string(e)
}
