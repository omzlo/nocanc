package webui

import (
	"github.com/omzlo/nocanc/helper"
	"github.com/omzlo/nocand/socket"
	"net/http"
)

var (
	DeviceInfo *socket.DeviceInformationEvent = nil
)

func on_device_information_event(conn *socket.EventConn, e socket.Eventer) error {
	DeviceInfo = e.(*socket.DeviceInformationEvent)
	return nil
}

func device_info_index(w http.ResponseWriter, req *http.Request, params *Parameters) {
	if DeviceInfo == nil {
		ErrorSend(w, req, helper.NotFound("No device information available"))
		return
	}

	JsonSend(w, req, DeviceInfo)
}
