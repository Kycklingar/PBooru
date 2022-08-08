package handlers

import (
	"fmt"
	"log"
	"net/http"

	DM "github.com/kycklingar/PBooru/DataManager"
)

func testHandler(w http.ResponseWriter, r *http.Request) {
	var p = DM.NewPost()

	p.ID = 1
	err := p.QMul(
		DM.DB,
		DM.PFCid,
		DM.PFMime,
		DM.PFDeleted,
		DM.PFSize,
		DM.PFDimension,
		DM.PFScore,
		DM.PFChecksums,
		DM.PFDescription,
		DM.PFThumbnails,
	)
	if err != nil {
		log.Println(err)
		return
	}

	fmt.Println(*p)

}
