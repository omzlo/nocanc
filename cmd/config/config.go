package config

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/omzlo/clog"
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
		s = append(s, fmt.Sprintf("%d::%s", item.Pin, item.Channel))
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

func superSplit(s string) []string {
	var result []string
	var item string

	escape := false
	for _, c := range s {
		if c == '{' {
			escape = true
		}
		if c == '}' {
			escape = false
		}
		if c == ',' && !escape {
			result = append(result, item)
			item = ""
		} else {
			item += string(c)
		}
	}
	if item != "" {
		result = append(result, item)
	}
	return result
}

type MqttAssoc struct {
	Channel   string
	Transform string
	Topic     string
}

func (ma *MqttAssoc) Set(s string) error {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return fmt.Errorf("Mqtt associations must contain strings seperated by ':'")
	}
	ma.Channel = parts[0]
	ma.Transform = parts[1]
	if len(parts[2]) == 0 {
		ma.Topic = parts[0]
	} else {
		ma.Topic = parts[2]
	}
	return nil
}

type MqttMap []*MqttAssoc

func (mm *MqttMap) Set(s string) error {
	*mm = nil

	v := superSplit(s)
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
		s = append(s, fmt.Sprintf("%s:%s:%s", item.Channel, item.Transform, item.Topic))
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
	ClientId    string  `toml:"client-id"`
	MqttServer  string  `toml:"mqtt-server"`
	Publishers  MqttMap `toml:"publishers"`
	Subscribers MqttMap `toml:"subscribers"`
}

type WebuiConfiguration struct {
	WebServer string `toml:"web-server"`
	Refresh   uint   `toml:"refresh"`
}

type Configuration struct {
	EventServer       string `toml:"event-server"`
	AuthToken         string `toml:"auth-token"`
	DownloadSizeLimit uint   `toml:"download-size-limit"`
	Blynk             BlynkConfiguration
	Mqtt              MqttConfiguration
	Webui             WebuiConfiguration
	CheckForUpdates   bool              `toml:"check-for-updates"`
	UpdateUrl         string            `toml:"update-url"`
	LogTerminal       string            `toml:"log-terminal"`
	LogLevel          clog.LogLevel     `toml:"log-level"`
	LogFile           *helpers.FilePath `toml:"log-file"`
	OnUpdate          bool              `toml:"on-update"`
}

var DefaultSettings = Configuration{
	EventServer:       ":4242",
	AuthToken:         "missing-password",
	DownloadSizeLimit: (1 << 32) - 1,
	Blynk: BlynkConfiguration{
		BlynkServer: blynk.BLYNK_ADDRESS,
		BlynkToken:  "missing-token",
	},
	Mqtt: MqttConfiguration{
		ClientId:   "",
		MqttServer: "mqtt://localhost",
	},
	Webui: WebuiConfiguration{
		WebServer: "localhost:8080",
		Refresh:   5000,
	},
	CheckForUpdates: true,
	UpdateUrl:       "https://www.omzlo.com/software_update",
	LogLevel:        clog.INFO,
	LogTerminal:     "plain",
	LogFile:         helpers.NewFilePath(),
	OnUpdate:        false,
}

var Settings = DefaultSettings

var DefaultConfigFile *helpers.FilePath = helpers.HomeDir().Append(".nocanc.conf")

func loadFile(file_path *helpers.FilePath) (bool, error) {
	if !file_path.Exists() {
		// no config file found, continue normally.
		return false, nil
	}

	if _, err := toml.DecodeFile(file_path.String(), &Settings); err != nil {
		return true, err
	}

	return true, nil
}

func LoadFile(fname string) (bool, error) {
	return loadFile(helpers.NewFilePath(fname))
}

func LoadDefault() (bool, error) {
	return loadFile(DefaultConfigFile)
}
