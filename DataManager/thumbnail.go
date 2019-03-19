package DataManager

import (
	"encoding/binary"
	"fmt"
	"image/png"
	"io"
	"log"

	"github.com/Nr90/imgsim"
	"github.com/kycklingar/PBooru/DataManager/image"
)

func makeThumbnail(file io.ReadSeeker, thumbnailSize int) (string, error) {
	b, err := image.MakeThumbnail(file, CFG.ThumbnailFormat, thumbnailSize)
	if err != nil {
		log.Println(err)
		return "", err
	}

	thumbHash, err := ipfsAdd(b)
	if err != nil {
		log.Println(err)
		return "", err
	}

	if CFG.UseMFS {
		err = mfsCP(fmt.Sprint(CFG.MFSRootDir, "thumbnails/", thumbnailSize, "/"), thumbHash, true)
	}

	return thumbHash, err
}

func ImageLookup(file io.ReadSeeker, distance int) ([]*Post, error) {
	hash := dHash(file)
	if hash == 0 {
		return nil, nil
	}

	type phash struct {
		post_id int
		h1      uint16
		h2      uint16
		h3      uint16
		h4      uint16
	}

	var ph phash
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, hash)
	ph.h1 = uint16(b[1]) | uint16(b[0])<<8
	ph.h2 = uint16(b[3]) | uint16(b[2])<<8
	ph.h3 = uint16(b[5]) | uint16(b[4])<<8
	ph.h4 = uint16(b[7]) | uint16(b[6])<<8

	rows, err := DB.Query("SELECT * FROM phash WHERE h1=$1 OR h2=$2 OR h3=$3 OR h4=$4 ORDER BY post_id DESC", ph.h1, ph.h2, ph.h3, ph.h4)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var phs []phash

	for rows.Next() {
		var phn phash
		if err = rows.Scan(&phn.post_id, &phn.h1, &phn.h2, &phn.h3, &phn.h4); err != nil {
			return nil, err
		}
		phs = append(phs, phn)
	}
	f := func(a phash) imgsim.Hash {
		return imgsim.Hash(uint64(a.h1)<<16 | uint64(a.h2)<<32 | uint64(a.h3)<<48 | uint64(a.h4)<<64)
	}

	var posts []*Post
	hasha := f(ph)

	for _, h := range phs {
		hashb := f(h)
		if imgsim.Distance(hasha, hashb) < distance {
			pst := NewPost()
			pst.ID = h.post_id
			posts = append(posts, pst)
		}
	}

	return posts, nil
}

func dHash(file io.ReadSeeker) uint64 {
	b, err := image.MakeThumbnail(file, "png", 1024)
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
