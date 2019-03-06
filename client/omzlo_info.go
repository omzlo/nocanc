package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/omzlo/clog"
	"net/http"
	"time"
)

type OmzloNews struct {
	Loaded bool   `json:"loaded,omitempty"`
	Html   string `json:"html"`
}

var LatestNews OmzloNews

var http_client = &http.Client{Timeout: 10 * time.Second}

func UpdateLatestNews(version string, os string, arch string) {

	di, cerr := GetDeviceInformation()
	if cerr != nil {
		clog.Warning("Could not get device information: %s.", cerr.Error)
		return
	}

	url := fmt.Sprintf("https://www.omzlo.com/software_update?i=%s&cv=%s&o=%s&a=%s&c=webui",
		base64.StdEncoding.EncodeToString(di.ChipId[:]),
		version,
		os,
		arch)

	r, err := http_client.Get(url)
	if err != nil {
		clog.Warning("Could not get latest news from omzlo.com (Set check-for-updates to 'false' in your configuration to dissable check for updates.)")
		return
	}
	defer r.Body.Close()

	err = json.NewDecoder(r.Body).Decode(&LatestNews)
	if err != nil {
		clog.Warning("Could not decode latest news from omzlo.com in JSON: %s.", err.Error())
		return
	}
	LatestNews.Loaded = true
	clog.Info("Loaded latest news from omzlo.com")
}
