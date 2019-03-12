package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"

	DM "github.com/kycklingar/PBooru/DataManager"
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

	migrateMfs := flag.Bool("migrate-mfs", false, "Migrate all files and thumbnails to the mfs.")
	initConfig := flag.Bool("init-cfg", false, "Initialize the configfile and exit.")
	configFilePath := flag.String("cfg", "config.cfg", "Load config file.")
	generateThumbnails := flag.Int("gen-thumbs", 0, "Generate (missing) thumbnails for this size")
	generateThumbnail := flag.Int("gen-thumb", 0, "Generate thumbnails for this post id")
	checkThumbSupport := flag.Bool("thumb-support", false, "Check for installed thumbnailing software")
	calcChecksums := flag.Bool("calc-checksums", false, "Calculate the checksums of all posts")
	generateFileSize := flag.Bool("gen-filesize", false, "Generate file sizes on posts with 0 filesize")
	generateFileDim := flag.Bool("gen-dimensions", false, "Generate file dimensions")

	flag.Parse()

	if *checkThumbSupport {
		DM.ThumbnailerInstalled()
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

	if *migrateMfs {
		DM.MigrateMfs()
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
