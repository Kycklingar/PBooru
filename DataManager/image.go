package DataManager

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"io"
	"log"
	"os/exec"
	"strings"

	"github.com/Nr90/imgsim"
)

func makeThumbnail(file io.Reader, mime string, thumbnailSize int) (string, error) {
	var err error
	if !strings.Contains(mime, "image") {
		return "", nil
	}

	b, err := magickResize(file, CFG.ThumbnailFormat, thumbnailSize)
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

	var b bytes.Buffer
	var err error
	command.Stdout = &b

	err = command.Run()
	if err != nil {
		log.Println(err)
		return nil, err
	}

	if len(b.Bytes()) <= 0 {
		return nil, errors.New("nolength buffer")
	}

	return &b, nil
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
