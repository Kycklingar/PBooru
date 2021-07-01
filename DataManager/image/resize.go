package image

import (
	"io"
	"log"

	"github.com/gabriel-vasile/mimetype"
)

var thumbnailers []Thumbnailer

// Populate with default thumbnailers
func init() {
	thumbnailers = append(thumbnailers, ImageMagickThumbnailer(), FFmpegThumbnailer())
}

func Resize(input io.ReadSeeker, format Format) (io.ReadSeekCloser, error) {
	mime, err := mimetype.DetectReader(input)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	if _, err = input.Seek(0, 0); err != nil {
		return nil, err
	}

	if format.ResizeFunc == nil {
		format.ResizeFunc = ShrinkKeepAspect
	}

	for _, tn := range thumbnailers {
		if tn.Accepts(mime.String()) {
			return tn.Resize(input, format)
		}
	}

	return nil, ErrNoThumbnailer(mime.String())
}
