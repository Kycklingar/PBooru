package DataManager

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"log"
	"strings"

	"github.com/Nr90/imgsim"
	"gopkg.in/gographics/imagick.v3/imagick"
)

func makeThumbnail(file io.Reader, mime string, thumbnailSize int) (string, error) {

	if !strings.Contains(mime, "image") {
		return "", nil
	}

	var b bytes.Buffer
	b.ReadFrom(file)

	mw := imagick.NewMagickWand()
	defer mw.Destroy()
	var err error

	if err = mw.ReadImageBlob(b.Bytes()); err != nil {
		log.Println(err)
		return "", err
	}

	if mw.GetNumberImages() > 1 {
		nmw := mw.CoalesceImages()
		mw.Destroy()
		mw = nmw
	}
	iw := mw.GetImageWidth()
	ih := mw.GetImageHeight()
	var width = iw
	var height = ih
	if iw >= uint(thumbnailSize) && ih >= uint(thumbnailSize) {
		if iw > ih {
			width = uint(thumbnailSize)
			height = uint(float32(ih) / float32(iw) * float32(thumbnailSize))
		} else if iw < ih {
			height = uint(thumbnailSize)
			width = uint(float32(iw) / float32(ih) * float32(thumbnailSize))
		}
	}

	if err = mw.SetImageFormat(CFG.ThumbnailFormat); err != nil {
		log.Println(err)
		return "", err
	}

	if err = mw.ResizeImage(width, height, imagick.FILTER_LANCZOS2); err != nil {
		log.Println(err, width, height, iw, ih)
		return "", err
	}

	if err = mw.SetImageCompressionQuality(85); err != nil {
		log.Println(err)
		return "", err
	}

	w := bytes.NewReader(mw.GetImageBlob())
	if err != nil {
		return "", err
	}

	thumbHash, err := ipfsAdd(w)
	if err != nil {
		log.Println(err)
		return "", err
	}

	err = mfsCP(fmt.Sprint(CFG.MFSRootDir, "thumbnails/", thumbnailSize, "/"), thumbHash, true)

	return thumbHash, err
}

func dHash(file io.Reader) uint64 {
	mw := imagick.NewMagickWand()

	var err error
	var b bytes.Buffer
	b.ReadFrom(file)

	if err = mw.ReadImageBlob(b.Bytes()); err != nil {
		log.Println(err)
		return 0
	}

	if err = mw.SetImageFormat("PNG"); err != nil {
		log.Println(err)
		return 0
	}
	f := bytes.NewReader(mw.GetImageBlob())

	img, _, err := image.Decode(f)
	if err != nil {
		log.Println(err)
		return 0
	}
	hash := imgsim.DifferenceHash(img)
	return uint64(hash)
}
