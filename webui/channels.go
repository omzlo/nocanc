package webui

import (
	"encoding/json"
	"github.com/omzlo/clog"
	"github.com/omzlo/nocanc/client"
	"github.com/omzlo/nocand/models/nocan"
	"io/ioutil"
	"net/http"
	"strconv"
)

func channels_index(w http.ResponseWriter, req *http.Request, params *Parameters) {
	cl, err := client.ListChannels()

	if err != nil {
		ErrorSend(w, req, err)
		return
	}

	JsonSend(w, req, cl)
	return
}

func channels_show(w http.ResponseWriter, req *http.Request, params *Parameters) {
	c, ok := parseChannel(w, req, params)
	if !ok {
		return
	}
	cu, cerr := client.GetChannel(c)

	if cerr != nil {
		ErrorSend(w, req, cerr)
		return
	}
	JsonSend(w, req, cu)
}

func channels_update(w http.ResponseWriter, req *http.Request, params *Parameters) {
	c, ok := parseChannel(w, req, params)
	if !ok {
		return
	}
	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		ErrorSend(w, req, client.InternalServerError("Failed to read request body:"+err.Error()))
		return
	}

	var value struct {
		Value string `json:"value"`
	}
	err = json.Unmarshal(b, &value)
	if err != nil {
		clog.Error("Got: %s", err)
		ErrorSend(w, req, client.BadRequest("JSON error: "+err.Error()))
		clog.DebugXX("JSON Data: %s", b)
		return
	}
	cerr := client.UpdateChannel(c, "", []byte(value.Value))
	if err != nil {
		ErrorSend(w, req, cerr)
		return
	}
}

func parseChannel(w http.ResponseWriter, req *http.Request, params *Parameters) (nocan.ChannelId, bool) {
	channelId, err := strconv.ParseUint(params.Value["id"], 0, 8)
	if err != nil {
		ErrorSend(w, req, client.BadRequest(err))
		return 0, false
	}
	return nocan.ChannelId(channelId), true
}
