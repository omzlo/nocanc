package config

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/omzlo/goblynk"
	"github.com/omzlo/nocand/models/helpers"
	"strconv"
	"strings"
)

/****/

type BlynkAssoc struct {
	Pin     uint
	Channel string
}

func (ba *BlynkAssoc) Set(s string) error {
	parts := strings.SplitN(s, "::", 2)
	pin, err := strconv.ParseUint(parts[0], 0, 32)
	if err != nil {
		return err
	}
	channel := parts[1]
	ba.Pin = uint(pin)
	ba.Channel = channel
	return nil

}

/*
func (ba *BlynkAssoc) UnmarshalText(text []byte) error {
	return ba.Set(string(text))
}
*/

type BlynkMap []*BlynkAssoc

func (bl *BlynkMap) Set(s string) error {
	*bl = nil

	v := strings.Split(s, ",")
	for _, item := range v {
		ba := new(BlynkAssoc)

		if err := ba.Set(item); err != nil {
			return err
		}
		*bl = append(*bl, ba)
	}
	return nil
}

func (bl BlynkMap) String() string {
	var s []string

	for _, item := range bl {
		s = append(s, fmt.Sprintf("%i:%s", item.Pin, item.Channel))
	}
	return strings.Join(s, ",")
}

type BlynkList []string

func (bl *BlynkList) Set(s string) error {
	*bl = strings.Split(s, ",")
	return nil
}

func (bl BlynkList) String() string {
	return strings.Join(bl, ",")
}

/***/

type BlynkConfiguration struct {
	BlynkServer string    `toml:"blynk-server"`
	BlynkToken  string    `toml:"blynk-token"`
	Readers     BlynkMap  `toml:"readers"`
	Writers     BlynkMap  `toml:"writers"`
	Notifiers   BlynkList `toml:"notifiers"`
}

type Configuration struct {
	EventServer       string `toml:"event-server"`
	AuthToken         string `toml:"auth-token"`
	DownloadSizeLimit uint   `toml:"download-size-limit"`
	Blynk             BlynkConfiguration
	CheckForUpdates   bool `toml:"check-for-updates"`
}

var Settings = Configuration{
	EventServer:       ":4242",
	AuthToken:         "missing-password",
	DownloadSizeLimit: (1 << 32) - 1,
	Blynk: BlynkConfiguration{
		BlynkServer: blynk.BLYNK_ADDRESS,
		BlynkToken:  "missing-token",
	},
	CheckForUpdates: true,
}

func Load() error {

	fn, err := helpers.LocateFile(helpers.HomeDir(), ".nocanc.conf")

	if err != nil {
		// no config file found, continue normally.
		return nil
	}

	if _, err := toml.DecodeFile(fn, &Settings); err != nil {
		return err
	}

	return nil
}
