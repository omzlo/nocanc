package webui

import (
	"github.com/omzlo/nocanc/client"
	"net/http"
)

func power_status_index(w http.ResponseWriter, req *http.Request, params *Parameters) {
	ps, err := client.GetPowerStatus()

	if err != nil {
		ErrorSend(w, req, err)
		return
	}

	JsonSend(w, req, ps)
}
