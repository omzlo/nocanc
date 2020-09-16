package webui

import (
	"github.com/omzlo/nocanc/helper"
	"net/http"
)

func news_index(w http.ResponseWriter, req *http.Request, params *Parameters) {
	JsonSend(w, req, helper.LatestNews)
	return
}
