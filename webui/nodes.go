package webui

import (
	"fmt"
	"github.com/omzlo/nocanc/helper"
	"github.com/omzlo/nocanc/intelhex"
	"github.com/omzlo/nocand/models/nocan"
	"github.com/omzlo/nocand/socket"
	"net/http"
	"strconv"
)

var NodeList *socket.NodeListEvent

func on_node_list_event(conn *socket.EventConn, e socket.Eventer) error {
	NodeList = e.(*socket.NodeListEvent)
	return nil
}

func on_node_update_event(conn *socket.EventConn, e socket.Eventer) error {
	nu := e.(*socket.NodeUpdateEvent)
	if NodeList == nil {
		NodeList = socket.NewNodeListEvent()
	}
	for i, node := range NodeList.Nodes {
		if node.NodeId == nu.NodeId {
			NodeList.Nodes[i] = nu
			return nil
		}
	}
	NodeList.Append(nu)
	return nil
}

func nodes_index(w http.ResponseWriter, req *http.Request, params *Parameters) {
	JsonSend(w, req, NodeList)
	return
}

func nodes_show(w http.ResponseWriter, req *http.Request, params *Parameters) {
	nodeId, err := strconv.ParseUint(params.Value["id"], 0, 8)
	if err != nil {
		ErrorSend(w, req, helper.BadRequest(err))
		return
	}

	for _, node := range NodeList.Nodes {
		if node.NodeId == nocan.NodeId(nodeId) {
			JsonSend(w, req, node)
			return
		}
	}

	ErrorSend(w, req, helper.NotFound(fmt.Sprintf("Node %d does not exist", nodeId)))
}

func nodes_upload(w http.ResponseWriter, req *http.Request, params *Parameters) {
	nodeId, err := strconv.ParseUint(params.Value["id"], 0, 8)
	if err != nil {
		ErrorSend(w, req, helper.BadRequest(err))
		return
	}

	req.ParseMultipartForm(512 * 1024)
	file, _, err := req.FormFile("firmware")
	if err != nil {
		ErrorSend(w, req, helper.BadRequest(err))
		return
	}
	ihex := intelhex.New()
	err = ihex.Load(file)
	file.Close()
	if err != nil {
		ErrorSend(w, req, helper.BadRequest("iHex parser: "+err.Error()))
		return
	}

	job, cerr := helper.UploadFirmware(NocanClient, nocan.NodeId(nodeId), ihex, nil)
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

func nodes_reboot(w http.ResponseWriter, req *http.Request, params *Parameters) {
	nodeId, err := strconv.ParseUint(params.Value["id"], 0, 8)
	if err != nil {
		ErrorSend(w, req, helper.BadRequest(err))
		return
	}
	force := params.Value["force"] == "true"

	if err := NocanClient.Send(socket.NewNodeRebootRequestEvent(nocan.NodeId(nodeId), force)); err != nil {
		ErrorSend(w, req, helper.InternalServerError(err))
		return
	}
	JsonSendWithStatus(w, req, nil, http.StatusNoContent)
}
