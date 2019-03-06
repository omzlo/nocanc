package webui

import (
	"fmt"
	"github.com/omzlo/nocanc/client"
	"github.com/omzlo/nocanc/intelhex"
	"net/http"
	"strconv"
)

func nodes_index(w http.ResponseWriter, req *http.Request, params *Parameters) {
	nl, err := client.ListNodes()

	if err != nil {
		ErrorSend(w, req, err)
		return
	}

	JsonSend(w, req, nl)
	return
}

func nodes_show(w http.ResponseWriter, req *http.Request, params *Parameters) {
	nodeId, err := strconv.ParseUint(params.Value["id"], 0, 8)
	if err != nil {
		ErrorSend(w, req, client.BadRequest(err))
		return
	}

	nu, cerr := client.GetNode(uint(nodeId))

	if cerr != nil {
		ErrorSend(w, req, cerr)
		return
	}

	JsonSend(w, req, nu)
	return
}

func nodes_upload(w http.ResponseWriter, req *http.Request, params *Parameters) {
	nodeId, err := strconv.ParseUint(params.Value["id"], 0, 8)
	if err != nil {
		ErrorSend(w, req, client.BadRequest(err))
		return
	}

	req.ParseMultipartForm(512 * 1024)
	file, _, err := req.FormFile("firmware")
	if err != nil {
		ErrorSend(w, req, client.BadRequest(err))
		return
	}
	ihex := intelhex.New()
	err = ihex.Load(file)
	file.Close()
	if err != nil {
		ErrorSend(w, req, client.BadRequest("iHex parser: "+err.Error()))
		return
	}

	job, cerr := client.UploadFirmware(uint(nodeId), ihex, nil)
	if cerr != nil {
		ErrorSend(w, req, cerr)
		return
	}

	var retval struct {
		Location string `json:"location"`
	}
	retval.Location = fmt.Sprintf("%s/jobs/%d", API_PREFIX, job.Id)
	w.Header().Add("Location", retval.Location)
	JsonSendWithStatus(w, req, retval, http.StatusCreated)
}
