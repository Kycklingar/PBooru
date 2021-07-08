package image

import (
	"io"
	"io/ioutil"
	"os"
	"testing"
)

func TestResizeFiles(t *testing.T) {
	files, err := ioutil.ReadDir("test/inputs")
	if err != nil {
		t.Fatal(err)
	}

	for _, fi := range files {
		if err = testResizeFile(fi.Name()); err != nil {
			t.Error(fi.Name(), err)
		}
	}
}

func testResizeFile(filename string) error {
	f, err := os.Open("test/inputs/" + filename)
	if err != nil {
		return err
	}
	defer f.Close()

	res, err := Resize(f, Format{Width: 256, Height: 256, Quality: 90, Mime: "png"})
	if err != nil {
		return err
	}
	defer res.Close()

	o, err := os.Create("test/output/" + filename + ".png")
	if err != nil {
		return err
	}
	defer o.Close()

	_, err = io.Copy(o, res)
	if err != nil {
		return err
	}

	return nil
}
