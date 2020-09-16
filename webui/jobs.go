package webui

import (
	"fmt"
	"github.com/omzlo/nocanc/helper"
	"net/http"
	"strconv"
)

func jobs_show(w http.ResponseWriter, req *http.Request, params *Parameters) {
	jobId, err := strconv.ParseInt(params.Value["id"], 0, 8)
	if err != nil {
		ErrorSend(w, req, helper.BadRequest(err))
		return
	}

	job := helper.DefaultJobManager.FindById(int(jobId))

	if job == nil {
		ErrorSend(w, req, helper.NotFound(fmt.Sprintf("job %d does not exist", jobId)))
		return
	}

	JsonSend(w, req, job)
	return
}
