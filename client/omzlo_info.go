package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/omzlo/clog"
	"github.com/omzlo/nocand/models/device"
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
	var di *device.Info

	for {
		di_x, cerr := GetDeviceInformation()
		if cerr == nil {
			di = di_x
			break
		}
		clog.Warning("Could not get device information: %s.", cerr.Error)
		time.Sleep(1 * time.Hour)
	}

	start := time.Now()

	for {
		url := fmt.Sprintf("https://www.omzlo.com/software_update?i=%s&cv=%s&o=%s&a=%s&c=webui&u=%d",
			base64.StdEncoding.EncodeToString(di.ChipId[:]),
			version,
			os,
			arch,
			time.Since(start)/time.Second)

		r, err := http_client.Get(url)
		if err != nil {
			clog.Warning("Could not get latest news from omzlo.com (Set check-for-updates to 'false' in your configuration to dissable check for updates.)")
		} else {
			err = json.NewDecoder(r.Body).Decode(&LatestNews)
			r.Body.Close()

			if err != nil {
				clog.Warning("Could not decode latest news from omzlo.com in JSON: %s.", err.Error())
				return
			}
			LatestNews.Loaded = true
			clog.Info("Loaded latest news from omzlo.com")
		}
		time.Sleep(24 * time.Hour)
	}
}
