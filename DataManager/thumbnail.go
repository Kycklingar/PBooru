package DataManager

import (
	"errors"
	"image/png"
	"io"
	"log"

	"github.com/Nr90/imgsim"
	shell "github.com/ipfs/go-ipfs-api"
	"github.com/kycklingar/PBooru/DataManager/image"
)

func makeThumbnail(file io.ReadSeeker, size uint, quality int) (string, error) {
	img, err := image.Resize(
		file,
		image.Format{
			Width:   size,
			Height:  size,
			Mime:    CFG.ThumbnailFormat,
			Quality: quality,
		})
	if err != nil {
		log.Println(err)
		return "", err
	}
	defer img.Close()

	cid, err := ipfs.Add(
		io.NopCloser(img),
		shell.Pin(false),
		shell.CidVersion(1),
	)
	if err != nil {
		log.Println(err)
		return "", err
	}

	if cid == "" {
		return "", errors.New("cid is empty")
	}

	return cid, nil

}

func makeThumbnails(file io.ReadSeeker) ([]Thumb, error) {
	var largestThumbnailSize uint
	for _, size := range CFG.ThumbnailSizes {
		if largestThumbnailSize < size {
			largestThumbnailSize = size
		}
	}

	img, err := image.Resize(
		file,
		image.Format{
			Width:   largestThumbnailSize,
			Height:  largestThumbnailSize,
			Mime:    "PNG",
			Quality: CFG.ThumbnailQuality,
		})
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer img.Close()

	var thumbs []Thumb

	for _, size := range CFG.ThumbnailSizes {
		img.Seek(0, 0)
		thumbHash, err := makeThumbnail(img, size, CFG.ThumbnailQuality)
		if err != nil {
			return nil, err
		}

		thumbs = append(thumbs, Thumb{Cid: thumbHash, Size: int(size)})
	}

	return thumbs, nil
}

func ImageLookup(file io.ReadSeeker, distance int) ([]*Post, error) {
	hash, err := dHash(file)
	if err != nil {
		return nil, err
	}
	if hash == 0 {
		return nil, nil
	}

	var ph = phsFromHash(0, hash)

	rows, err := DB.Query("SELECT * FROM phash WHERE h1=$1 OR h2=$2 OR h3=$3 OR h4=$4 ORDER BY post_id DESC", ph.h1, ph.h2, ph.h3, ph.h4)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var phas []phs

	for rows.Next() {
		var phn phs
		if err = rows.Scan(&phn.postid, &phn.h1, &phn.h2, &phn.h3, &phn.h4); err != nil {
			return nil, err
		}
		phas = append(phas, phn)
	}

	var posts []*Post

	for _, h := range phas {
		if ph.distance(h) < distance {
			pst := NewPost()
			pst.ID = h.postid
			posts = append(posts, pst)
		}
	}

	return posts, nil
}

func dHash(file io.ReadSeeker) (imgsim.Hash, error) {
	imgf, err := image.Resize(
		file,
		image.Format{
			Width:   1024,
			Height:  1024,
			Mime:    "PNG",
			Quality: 75,
		},
	)
	if err != nil {
		log.Println(err)
		return 0, err
	}
	defer imgf.Close()

	img, err := png.Decode(imgf)
	if err != nil {
		log.Println(err)
		return 0, err
	}
	hash := imgsim.DifferenceHash(img)
	return hash, nil
}
