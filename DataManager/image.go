package DataManager

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/Nr90/imgsim"
	"github.com/zRedShift/mimemagic"
)

func makeThumbnail(file io.ReadSeeker, thumbnailSize int) (string, error) {
	var err error

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		log.Println(err)
		return "", err
	}

	mime := mimemagic.MatchMagic(buffer)

	//fmt.Println(mime.MediaType())
	file.Seek(0, 0)

	var b *bytes.Buffer

	switch mime.MediaType() {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		b, err = magickResize(file, CFG.ThumbnailFormat, thumbnailSize)
	case "application/pdf", "application/epub+zip":
		var m string
		if strings.Contains(mime.MediaType(), "pdf") {
			m = "pdf"
		} else if strings.Contains(mime.MediaType(), "epub") {
			m = "epub"
		}
		b, err = mupdf(file, m, CFG.ThumbnailFormat, thumbnailSize)
	default:
		return "", nil
	}

	if err != nil {
		log.Println(err)
		return "", err
	}

	thumbHash, err := ipfsAdd(b)
	if err != nil {
		log.Println(err)
		return "", err
	}

	err = mfsCP(fmt.Sprint(CFG.MFSRootDir, "thumbnails/", thumbnailSize, "/"), thumbHash, true)

	return thumbHash, err
}

func magickResize(file io.Reader, format string, size int) (*bytes.Buffer, error) {
	args := []string{
		"-[0]",
		"-quality",
		"75",
		"-strip",
		"-resize",
		fmt.Sprintf("%dx%d\\>", size, size),
		fmt.Sprintf("%s:-", format),
	}
	command := exec.Command("magick", args...)

	command.Stdin = file

	var b, er bytes.Buffer
	var err error
	command.Stdout = &b
	command.Stderr = &er

	err = command.Run()
	if err != nil {
		log.Println(b.String(), er.String(), err)
		return nil, err
	}

	if len(b.Bytes()) <= 0 {
		return nil, errors.New("nolength buffer")
	}

	return &b, nil
}

func mupdf(file io.Reader, mime, format string, size int) (*bytes.Buffer, error) {
	tmpdir, err := ioutil.TempDir("", "pbooru-tmp")
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer os.RemoveAll(tmpdir)

	var tmpbuf bytes.Buffer
	tmpbuf.ReadFrom(file)

	err = ioutil.WriteFile(fmt.Sprintf("%s/file.%s", tmpdir, mime), tmpbuf.Bytes(), 0660)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	args := []string{
		"draw",
		"-o",
		"",
		"-F",
		"png",
		fmt.Sprintf("%s/file.%s", tmpdir, mime),
		"1",
	}

	cmd := exec.Command("mutool", args...)

	var b, er bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &er

	err = cmd.Run()
	if err != nil {
		log.Println(b.String(), er.String(), err)
		return nil, err
	}

	f := bytes.NewReader(b.Bytes())
	return magickResize(f, format, size)
}

func dHash(file io.Reader) uint64 {
	b, err := magickResize(file, "png", 1024)
	if err != nil {
		log.Println(err)
		return 0
	}
	img, err := png.Decode(b)
	if err != nil {
		log.Println(err)
		return 0
	}
	hash := imgsim.DifferenceHash(img)
	return uint64(hash)
}
