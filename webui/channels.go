package webui

import (
	"encoding/json"
	"github.com/omzlo/clog"
	"github.com/omzlo/nocanc/helper"
	"github.com/omzlo/nocand/models/nocan"
	"github.com/omzlo/nocand/socket"
	"io/ioutil"
	"net/http"
	"strconv"
)

var ChannelList *socket.ChannelListEvent

func on_channel_list_event(conn *socket.EventConn, e socket.Eventer) error {
	ChannelList = e.(*socket.ChannelListEvent)
	return nil
}

func on_channel_update_event(conn *socket.EventConn, e socket.Eventer) error {
	cu := e.(*socket.ChannelUpdateEvent)
	if ChannelList == nil {
		ChannelList = socket.NewChannelListEvent()
	}
	for i, channel := range ChannelList.Channels {
		if channel.ChannelId == cu.ChannelId {
			ChannelList.Channels[i] = cu
			return nil
		}
	}
	ChannelList.Append(cu)
	return nil
}

func channels_index(w http.ResponseWriter, req *http.Request, params *Parameters) {
	JsonSend(w, req, ChannelList)
	return
}

func channels_show(w http.ResponseWriter, req *http.Request, params *Parameters) {
	c, ok := parseChannel(w, req, params)
	if !ok {
		return
	}
	for _, channel := range ChannelList.Channels {
		if channel.ChannelId == c {
			JsonSend(w, req, channel)
			return
		}
	}
	ErrorSend(w, req, helper.NotFound(nil))
}

func channels_update(w http.ResponseWriter, req *http.Request, params *Parameters) {
	c, ok := parseChannel(w, req, params)
	if !ok {
		return
	}
	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		ErrorSend(w, req, helper.InternalServerError("Failed to read request body:"+err.Error()))
		return
	}

	var value struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(b, &value); err != nil {
		clog.Error("Got: %s", err)
		ErrorSend(w, req, helper.BadRequest("JSON error: "+err.Error()))
		clog.DebugXX("JSON Data: %s", b)
		return
	}

	if err := NocanClient.Send(socket.NewChannelUpdateEvent("", c, socket.CHANNEL_UPDATED, []byte(value.Value))); err != nil {
		ErrorSend(w, req, helper.InternalServerError(err))
		return
	}
	JsonSendWithStatus(w, req, nil, http.StatusNoContent)
}

func parseChannel(w http.ResponseWriter, req *http.Request, params *Parameters) (nocan.ChannelId, bool) {
	channelId, err := strconv.ParseUint(params.Value["id"], 0, 8)
	if err != nil {
		ErrorSend(w, req, helper.BadRequest(err))
		return 0, false
	}
	return nocan.ChannelId(channelId), true
}
