package DataManager

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"strings"

	"github.com/Nr90/imgsim"

	"github.com/nfnt/resize"
)

const thumbnailSize = 1024

func makeThumbnail(file io.Reader, mime string) (string, error) {

	if !strings.Contains(mime, "image") {
		return "NT", nil
	}

	img, _, err := image.Decode(file)
	if err != nil {
		fmt.Println("error decoding")
		return "", err
	}

	var width, height int

	rec := img.Bounds()

	if rec.Dx() < thumbnailSize && rec.Dy() < thumbnailSize {
		width = rec.Dx()
		height = rec.Dy()
	} else if rec.Dx() > rec.Dy() {
		width = thumbnailSize
		height = 0
	} else {
		width = 0
		height = thumbnailSize
	}

	thumb := resize.Resize(uint(width), uint(height), img, resize.Lanczos3)

	var w bytes.Buffer

	err = jpeg.Encode(&w, thumb, &jpeg.Options{Quality: 85})
	if err != nil {
		return "", err
	}

	thumbHash, err := ipfsAdd(&w)
	if err != nil {
		log.Println(err)
		return "", err
	}

	err = mfsCP(fmt.Sprint(CFG.MFSRootDir, "thumbnails/", thumbnailSize, "/"), thumbHash, true)

	return thumbHash, err
}

func dHash(file io.Reader) uint64 {
	img, _, err := image.Decode(file)
	if err != nil {
		log.Println(err)
		return 0
	}

	hash := imgsim.DifferenceHash(img)
	return uint64(hash)
}
