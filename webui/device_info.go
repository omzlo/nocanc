package webui

import (
	"github.com/omzlo/nocanc/client"
	"net/http"
)

func device_info_index(w http.ResponseWriter, req *http.Request, params *Parameters) {
	di, err := client.GetDeviceInformation()

	if err != nil {
		ErrorSend(w, req, err)
		return
	}

	JsonSend(w, req, di)
	return
}
