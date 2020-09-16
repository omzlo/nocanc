package webui

import (
	"github.com/omzlo/nocanc/helper"
	"github.com/omzlo/nocand/socket"
	"net/http"
)

var SystemProperties *socket.SystemPropertiesEvent

func on_system_properties_update_event(conn *socket.EventConn, e socket.Eventer) error {
	SystemProperties = e.(*socket.SystemPropertiesEvent)
	return nil
}

func system_properties_index(w http.ResponseWriter, req *http.Request, params *Parameters) {
	if SystemProperties == nil {
		ErrorSend(w, req, helper.NotFound(nil))
		return
	}

	JsonSend(w, req, SystemProperties)
}
