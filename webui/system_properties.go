package webui

import (
	"github.com/omzlo/nocanc/client"
	"net/http"
)

func system_properties_index(w http.ResponseWriter, req *http.Request, params *Parameters) {
	sp, err := client.GetSystemProperties()

	if err != nil {
		ErrorSend(w, req, err)
		return
	}

	JsonSend(w, req, sp)
	return
}
