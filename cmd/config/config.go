package config

import (
	"github.com/BurntSushi/toml"
	"github.com/omzlo/nocand/models/helpers"
)

type Configuration struct {
	EventServer string
	AuthToken   string
}

var Settings = Configuration{
	EventServer: ":4242",
	AuthToken:   "missing-password",
}

func Load() error {

	fn, err := helpers.LocateDotFile("nocanc.conf")

	if err != nil {
		return err
	}

	if _, err := toml.DecodeFile(fn, &Settings); err != nil {
		return err
	}

	return nil
}
