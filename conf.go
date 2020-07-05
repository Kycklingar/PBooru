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
	c.IPFSAPI = "http://localhost:5001"
	c.HCfg.Default()
	c.DBCfg.Default()
}

func exeConf(filePath string) config {
	var conf config

	conf.Default()

	if err := conf.loadConfigFile(filePath); err != nil {
		log.Fatal("Failed to load config file:", err)
	}

	if err := conf.saveConfigFile(filePath); err != nil {
		log.Fatal("Failed to save config file:", err)
	}

	return conf
}

func (c *config) loadConfigFile(filePath string) error {
	file, err := c.openConfigFile(filePath, os.O_RDWR)
	if err != nil {
		log.Println(err)
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&c)
	if err == io.EOF {
		return nil
	}

	return err
}

func (c *config) openConfigFile(filePath string, mode int) (*os.File, error) {
	file, err := os.OpenFile(filePath, mode, 0600)
	if err != nil {
		// If the file doesn't exist. Create it
		if !os.IsNotExist(err) {
			return nil, err
		}
		file, err = os.OpenFile(filePath, os.O_CREATE, 0600)
		if err != nil {
			return nil, err
		}
	}

	return file, nil
}

func (c *config) saveConfigFile(filePath string) error {
	file, err := c.openConfigFile(filePath, os.O_TRUNC|os.O_RDWR)
	if err != nil {
		log.Println(err)
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "	")
	enc.Encode(c)

	return nil
}
