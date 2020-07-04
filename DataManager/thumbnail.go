package DataManager

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"io"
	"log"

	"github.com/Nr90/imgsim"
	"github.com/kycklingar/PBooru/DataManager/image"
)

func makeThumbnail(parentCid string, file io.ReadSeeker, size, quality int) (string, error) {
	b, err := image.MakeThumbnail(file, CFG.ThumbnailFormat, size, quality)
	if err != nil {
		log.Println(err)
		return "", err
	}

	thumbHash, err := ipfsAdd(b)
	if err != nil {
		log.Println(err)
		return "", err
	}

	if thumbHash == "" {
		return "", errors.New("thumbhash is empty")
	}

	if CFG.UseMFS {
		if err = mfsCP(parentCid, fmt.Sprint(CFG.MFSRootDir, "thumbnails/", size, "/"), true); err != nil {
			log.Println(err)
			return "", err
		}
	}

	return thumbHash, nil

}

func makeThumbnails(parentCid string, file io.ReadSeeker) ([]Thumb, error) {
	var largestThumbnailSize int
	for _, size := range CFG.ThumbnailSizes {
		if largestThumbnailSize < size {
			largestThumbnailSize = size
		}
	}

	b, err := image.MakeThumbnail(file, "PNG", largestThumbnailSize, CFG.ThumbnailQuality)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	var f = bytes.NewReader(b.Bytes())

	var thumbs []Thumb

	for _, size := range CFG.ThumbnailSizes {
		f.Seek(0, 0)
		thumbHash, err := makeThumbnail(parentCid, f, size, CFG.ThumbnailQuality)
		if err != nil {
			return nil, err
		}

		thumbs = append(thumbs, Thumb{Hash: thumbHash, Size: size})
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
	b, err := image.MakeThumbnail(file, "png", 1024, 75)
	if err != nil {
		log.Println(err)
		return 0, err
	}

	img, err := png.Decode(b)
	if err != nil {
		log.Println(err)
		return 0, err
	}
	hash := imgsim.DifferenceHash(img)
	return hash, nil
}
