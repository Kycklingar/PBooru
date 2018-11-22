package main

import (
	"encoding/json"
	"io"
	"log"
	"os"

	DM "github.com/kycklingar/PBooru/DataManager"
	"github.com/kycklingar/PBooru/handlers"
)

type config struct {
	SSLCertPath  string
	SSLKeyPath   string
	HTTPAddress  string
	HTTPRedirect string
	HTTPSAddress string
	IPFSAPI      string

	HCfg  handlers.Config
	DBCfg DM.Config
}

func (c *config) Default() {
	c.HTTPAddress = ":80"
	c.IPFSAPI = "http://localhost:5001/api/v0/"
	c.HCfg.Default()
	c.DBCfg.Default()
}

func exeConf() config {
	var conf config
	conf.HCfg.IPFSDaemonMap = make(map[string]string)
	file, err := os.Open("config.cfg")
	if err != nil {
		if os.IsNotExist(err) {
			file, err := os.Create("config.cfg")
			if err != nil {
				log.Fatal("Error opening config file: ", err.Error())
			}
			defer file.Close()
			createConfigFile(&conf, file)
		} else {
			log.Fatal("Error opening config file: ", err.Error())
		}
	} else {
		defer file.Close()

		decoder := json.NewDecoder(file)

		err = decoder.Decode(&conf)
		decoder.DisallowUnknownFields()
		if err != nil {
			log.Fatal("Error decoding config: ", err.Error())
		}
	}

	return conf
}

func createConfigFile(c *config, w io.Writer) {
	c.Default()
	enc := json.NewEncoder(w)
	enc.SetIndent("", "	")
	enc.Encode(c)
}
