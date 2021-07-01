package dns

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/kycklingar/PBooru/DataManager/image"
	store "github.com/kycklingar/PBooru/DataManager/storage"
)

type Banner struct {
	Cid  string
	Type string
}

const dnsPath = "dns"

var (
	bannerFormat = image.Format{
		Width:      1024,
		Height:     256,
		Mime:       "WEBP",
		Quality:    90,
		ResizeFunc: bannerResizeFunc,
	}
	profileFormat = image.Format{
		Width:   128,
		Height:  128,
		Mime:    "WEBP",
		Quality: 90,
	}
)

func bannerResizeFunc(width, height uint) []string {
	return []string{
		"-resize",
		fmt.Sprintf("%dx%d^", width, height),
		"-gravity",
		"center",
		"-background",
		"none",
		"-extent",
		fmt.Sprintf("%dx%d", width, height),
	}
}

// Create or Update a new banner [profile, banner] and store it in the database
func NewBanner(file io.ReadSeeker, ipfs *shell.Shell, store store.Storage, db *sql.DB, creatorID int, bannerType string) error {
	var format image.Format
	switch bannerType {
	case "profile":
		format = profileFormat
	case "banner":
		format = bannerFormat
	default:
		return errors.New("Invalid banner type")
	}

	img, err := image.Resize(file, format)
	if err != nil {
		return err
	}
	defer img.Close()

	cid, err := ipfs.Add(
		io.NopCloser(img),
		shell.CidVersion(1),
		shell.Pin(false),
	)
	if err != nil {
		return err
	}

	err = store.Store(cid, path.Join(dnsPath, strconv.Itoa(creatorID), bannerType))
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		INSERT INTO dns_banners(creator_id, cid, banner_type)
		VALUES($1, $2, $3)
		`,
		creatorID,
		cid,
		bannerType,
	)
	if err != nil {
		return err
	}

	return nil
}

func GetBanners(db *sql.DB, creatorID int) ([]Banner, error) {
	rows, err := db.Query(`
		SELECT cid, banner_type
		FROM dns_banners
		WHERE creator_id = $1
		`,
		creatorID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var banners []Banner

	for rows.Next() {
		var b Banner
		if err = rows.Scan(&b.Cid, &b.Type); err != nil {
			return nil, err
		}

		banners = append(banners, b)
	}

	return banners, nil
}
