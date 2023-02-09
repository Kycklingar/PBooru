package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"

	DM "github.com/kycklingar/PBooru/DataManager"
	"github.com/kycklingar/PBooru/DataManager/image"
	apiv1 "github.com/kycklingar/PBooru/api/rest"
	h "github.com/kycklingar/PBooru/handlers"
)

var gConf config

//var gManager *DM.Manager = DM.GManager

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/favicon.ico")
}

func serveHiddenFiles(dir string) {
	files, err := ioutil.ReadDir("." + dir)
	if err != nil {
		panic(err)
	}

	for _, f := range files {
		//fmt.Println(dir + f.Name())
		serveHiddenFile(dir + f.Name())
	}
}

func serveHiddenFile(path string) {
	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "."+path)
	})
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	//fmt.Println(r.Host, r.RequestURI)
	http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusSeeOther)
}

func main() {

	migrateStore := flag.Int("migrate-store", -1, "Migrate all files and thumbnails to the configured store")
	initDir := flag.Bool("init-dir", false, "Initialize an ipfs directory of all files")
	initConfig := flag.Bool("init-cfg", false, "Initialize the configfile and exit.")
	configFilePath := flag.String("cfg", "config.cfg", "Load config file.")
	generateThumbnails := flag.Int("gen-thumbs", 0, "Generate (missing) thumbnails for this size")
	generateThumbnail := flag.Int("gen-thumb", 0, "Generate thumbnails for this post id")
	checkThumbSupport := flag.Bool("thumb-support", false, "Check for installed thumbnailing software")
	calcChecksums := flag.Bool("calc-checksums", false, "Calculate the checksums of all posts")
	generateFileSize := flag.Bool("gen-filesize", false, "Generate file sizes on posts with 0 filesize")
	generateFileDim := flag.Bool("gen-dimensions", false, "Generate file dimensions")
	upgradePostCids := flag.Bool("upgrade-post-cids", false, "Upgrade post cids to base32")
	genPears := flag.Bool("gen-pears", false, "Harvest apple tree")
	recalcApple := flag.Bool("recalc-appletree", false, "Recalculate appletree")
	genPhash := flag.Bool("gen-phash", false, "Generate phashes")
	tombstoneFile := flag.String("read-tombstone", "", "Read tombstones from a tombstone file")
	verifyFiles := flag.Int("verify-files", -1, "Verify the integrity of the files in ipfs")

	updateUserFlags := flag.Bool("update-user-flags", false, "Update user flags <old> <new>")

	flag.Parse()

	if *checkThumbSupport {
		image.ThumbnailerInstalled()
		return
	}

	log.SetFlags(log.Llongfile)

	gConf = exeConf(*configFilePath)
	h.CFG = &gConf.HCfg
	DM.CFG = &gConf.DBCfg

	if *initConfig {
		return
	}

	DM.Setup(gConf.IPFSAPI)

	go catchSignals()

	if *verifyFiles >= 0 {
		log.Println(DM.VerifyFileIntegrity(*verifyFiles))
		return
	}

	if *tombstoneFile != "" {
		log.Println(DM.UpdateTombstone(*tombstoneFile))
		return
	}

	if *genPears {
		if err := DM.GeneratePears(); err != nil {
			log.Fatal(err)
		}

		return
	}

	if *recalcApple {
		if err := DM.RecalculateAppletree(); err != nil {
			log.Fatal(err)
		}

		return
	}

	if *genPhash {
		log.Print(DM.GenPhash())
		return
	}

	if *upgradePostCids {
		DM.UpgradePostCids()
		return
	}

	if *updateUserFlags {
		oldStr := flag.Arg(0)
		newStr := flag.Arg(1)

		old, err := strconv.Atoi(oldStr)
		if err != nil {
			fmt.Println("<Old> expected a number, got:", oldStr)
			return
		}

		new, err := strconv.Atoi(newStr)
		if err != nil {
			fmt.Println("<New> expected a number, got:", newStr)
			return
		}

		if err := DM.UpdateUserFlags(new, old); err != nil {
			log.Fatal(err)
		}

		return
	}

	if *generateFileDim {
		DM.GenerateFileDimensions()
		return
	}

	if *calcChecksums {
		if err := DM.CalculateChecksums(); err != nil {
			log.Fatal(err)
		}
		return
	}

	if *generateFileSize {
		if err := DM.GenerateFileSizes(); err != nil {
			log.Fatal(err)
		}
		return
	}

	if *initDir {
		DM.InitDir()
		return
	}

	if *migrateStore >= 0 {
		DM.MigrateStore(*migrateStore)
		return
	}

	if *generateThumbnail > 0 {
		DM.GenerateThumbnail(*generateThumbnail)
		return
	}

	if *generateThumbnails > 0 {
		DM.GenerateThumbnails(*generateThumbnails)
		return
	}

	fs := http.FileServer(http.Dir("./static/"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.HandleFunc("/favicon.ico", faviconHandler)

	//serveHiddenFiles("/hidden/")

	for hk, hv := range h.Handlers {
		http.HandleFunc(hk, hv)
	}

	http.Handle("/api/v1/", http.StripPrefix("/api/v1", apiv1.Mux))

	if gConf.HTTPRedirect == "true" && gConf.HTTPSAddress != "" {
		if gConf.HTTPAddress == "" {
			gConf.HTTPAddress = ":80"
		}
		go log.Print(http.ListenAndServeTLS(gConf.HTTPSAddress, gConf.SSLCertPath, gConf.SSLKeyPath, nil))
		log.Print(http.ListenAndServe(gConf.HTTPAddress, http.HandlerFunc(redirectHandler)))
	} else {
		if gConf.HTTPAddress == "" && gConf.HTTPSAddress != "" {
			log.Print(http.ListenAndServeTLS(gConf.HTTPSAddress, gConf.SSLCertPath, gConf.SSLKeyPath, nil))
		} else {
			if gConf.HTTPSAddress != "" {
				go log.Print(http.ListenAndServeTLS(gConf.HTTPSAddress, gConf.SSLCertPath, gConf.SSLKeyPath, nil))
			}
			log.Print(http.ListenAndServe(gConf.HTTPAddress, nil))
		}
	}
	//http.HandleFunc("/static/", http.NotFound)

}

func catchSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	err := DM.Close()
	if err != nil {
		log.Println(err)
	}
	os.Exit(1)
}
