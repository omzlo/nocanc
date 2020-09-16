package helper

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/omzlo/clog"
	"github.com/omzlo/nocanc/cmd/config"
	"github.com/omzlo/nocand/socket"
	"net/http"
	"time"
)

type OmzloNews struct {
	Loaded bool   `json:"loaded,omitempty"`
	Html   string `json:"html"`
	Text   string `json:"text"`
}

var LatestNews OmzloNews

var http_client = &http.Client{Timeout: 10 * time.Second}

func UpdateLatestNews(client_type string, version string, os string, arch string, deviceInfo **socket.DeviceInformationEvent) {
	var chip_id string

	for {
		if *deviceInfo != nil {
			chip_id = base64.StdEncoding.EncodeToString((*deviceInfo).Information.ChipId[:])
			break
		}
		time.Sleep(10 * time.Second)
	}

	start := time.Now()
	for {
		LatestNews.Loaded = false

		url := fmt.Sprintf("%s?i=%s&cv=%s&o=%s&a=%s&c=%s&u=%d&t=nocanc",
			config.Settings.UpdateUrl,
			chip_id,
			version,
			os,
			arch,
			client_type,
			time.Since(start)/time.Second)

		clog.Info("Query to %s", url)

		r, err := http_client.Get(url)
		if err != nil {
			clog.Warning("Could not get latest news from %s (Set check-for-updates to 'false' in your configuration to dissable check for updates.)", config.Settings.UpdateUrl)
		} else {
			err = json.NewDecoder(r.Body).Decode(&LatestNews)
			r.Body.Close()

			if err != nil {
				clog.Warning("Could not decode latest news from %s in JSON: %s.", config.Settings.UpdateUrl, err.Error())
				return
			}
			LatestNews.Loaded = true
			clog.Info("Loaded latest news from omzlo.com")
		}
		if client_type == "cli" {
			return
		}
		time.Sleep(24 * time.Hour)
	}
}
