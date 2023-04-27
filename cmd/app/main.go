package main

import (
	"filemanager/internal/app/filemanager"
	"flag"
	"log"

	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"
)

var (
	configPath string
)

func init() {
	flag.StringVar(&configPath, "config-path", "configs/filemanager.toml", "path to config file")
}

func main() {
	flag.Parse()
	config := filemanager.NewConfig()
	_, err := toml.DecodeFile(configPath, config)
	if err != nil {
		log.Fatal(err)
	}

	logger := logrus.New()

	if err := filemanager.Start(config, logger); err != nil {
		log.Fatal(err)
	}
}
