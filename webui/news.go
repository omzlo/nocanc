package webui

import (
	"github.com/omzlo/nocanc/client"
	"net/http"
)

func news_index(w http.ResponseWriter, req *http.Request, params *Parameters) {
	JsonSend(w, req, client.LatestNews)
	return
}
