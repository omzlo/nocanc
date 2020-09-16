package webui

import (
	"github.com/omzlo/nocanc/helper"
	"github.com/omzlo/nocand/socket"
	"net/http"
)

var PowerStatus *socket.BusPowerStatusUpdateEvent

func on_power_status_update_event(conn *socket.EventConn, e socket.Eventer) error {
	PowerStatus = e.(*socket.BusPowerStatusUpdateEvent)
	return nil
}

func power_status_index(w http.ResponseWriter, req *http.Request, params *Parameters) {
	if PowerStatus == nil {
		ErrorSend(w, req, helper.NotFound(nil))
		return
	}
	JsonSend(w, req, PowerStatus.Status)
}
