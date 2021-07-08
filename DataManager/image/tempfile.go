package image

import (
	"io/ioutil"
	"os"
)

func tempFile() (*tmpFile, error) {
	f, err := ioutil.TempFile("", "pbooru-")
	if err != nil {
		return nil, err
	}

	return &tmpFile{
		file: f,
	}, nil
}

type tmpFile struct {
	file *os.File
}

func (f *tmpFile) Name() string { return f.file.Name() }

func (f *tmpFile) Read(p []byte) (int, error) { return f.file.Read(p) }

func (f *tmpFile) Write(p []byte) (int, error) { return f.file.Write(p) }

func (f *tmpFile) Seek(offset int64, whence int) (int64, error) { return f.file.Seek(offset, whence) }

func (f *tmpFile) Close() error {
	f.file.Close()
	return os.Remove(f.file.Name())
}
