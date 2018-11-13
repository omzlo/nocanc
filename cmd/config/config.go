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
		s = append(s, fmt.Sprintf("%i::%s", item.Pin, item.Channel))
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

type MqttAssoc struct {
	Channel string
	Topic   string
}

func (ma *MqttAssoc) Set(s string) error {
	parts := strings.SplitN(s, "::", 2)
	if len(parts) != 2 {
		return fmt.Errorf("Mqtt associations must contain two strings seperated by '::'")
	}
	ma.Channel = parts[0]
	if len(parts[1]) == 0 {
		ma.Topic = parts[0]
	} else {
		ma.Topic = parts[1]
	}
	return nil
}

type MqttMap []*MqttAssoc

func (mm *MqttMap) Set(s string) error {
	*mm = nil

	v := strings.Split(s, ",")
	for _, item := range v {
		ma := new(MqttAssoc)

		if err := ma.Set(item); err != nil {
			return err
		}
		*mm = append(*mm, ma)
	}
	return nil
}

func (mm MqttMap) String() string {
	var s []string

	for _, item := range mm {
		s = append(s, fmt.Sprintf("%s::%s", item.Channel, item.Topic))
	}
	return strings.Join(s, ",")
}

/***/

type BlynkConfiguration struct {
	BlynkServer string    `toml:"blynk-server"`
	BlynkToken  string    `toml:"blynk-token"`
	Readers     BlynkMap  `toml:"readers"`
	Writers     BlynkMap  `toml:"writers"`
	Notifiers   BlynkList `toml:"notifiers"`
}

type MqttConfiguration struct {
	ClientId   string  `toml:"client-id"`
	MqttServer string  `toml:"mqtt-server"`
	Publishers MqttMap `toml:"publishers"`
}

type Configuration struct {
	EventServer       string `toml:"event-server"`
	AuthToken         string `toml:"auth-token"`
	DownloadSizeLimit uint   `toml:"download-size-limit"`
	Blynk             BlynkConfiguration
	Mqtt              MqttConfiguration
	CheckForUpdates   bool   `toml:"check-for-updates"`
	LogTerminal       string `toml:"log-terminal"`
}

var Settings = Configuration{
	EventServer:       ":4242",
	AuthToken:         "missing-password",
	DownloadSizeLimit: (1 << 32) - 1,
	Blynk: BlynkConfiguration{
		BlynkServer: blynk.BLYNK_ADDRESS,
		BlynkToken:  "missing-token",
	},
	Mqtt: MqttConfiguration{
		ClientId:   "nocanc",
		MqttServer: "mqtt://localhost",
	},
	CheckForUpdates: true,
	LogTerminal:     "plain",
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
