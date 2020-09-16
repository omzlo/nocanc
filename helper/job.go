package helper

import (
	"errors"
	"github.com/omzlo/clog"
	//"github.com/omzlo/nocanc/intelhex"
	"sync"
	"time"
)

var (
	JOB_TIMEOUT_ERROR = errors.New("Job timed out")
)

type JobManager struct {
	Mutex sync.Mutex
	First *Job
	Last  *Job
	TopId int
}

type JobStatus uint

const (
	JOB_RUNNING JobStatus = iota
	JOB_SUCCESS
	JOB_ERROR
)

func (js JobStatus) MarshalJSON() ([]byte, error) {
	var s string

	switch js {
	case JOB_RUNNING:
		s = "running"
	case JOB_SUCCESS:
		s = "success"
	case JOB_ERROR:
		s = "error"
	default:
		s = "unknown"
	}
	return []byte(`"` + s + `"`), nil
}

type JobUpdater interface {
	Update(*Job)
}

type nop_updater int

var NopUpdater nop_updater

func (n nop_updater) Update(job *Job) {
	// do nothing
}

type Job struct {
	Id        int       `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Progress  float32   `json:"progress"`
	Status    JobStatus `json:"status"`
	Error     error     `json:"error,omitempty"`
	updater   JobUpdater
	manager   *JobManager
	next      *Job
}

func NewJobManager() *JobManager {
	jm := &JobManager{} // All defaults are OK
	go jm.Monitor()
	return jm
}

func (jm *JobManager) Monitor() {
	clog.DebugXX("Launching job monitor instance %p", jm)
	for {
		var cur *Job = nil

		jm.Mutex.Lock()
		for cur = jm.First; cur != nil; cur = cur.next {
			if time.Since(cur.UpdatedAt).Seconds() > 180 {
				clog.Warning("Job %d is inactive and will be dequeued.", cur.Id)
				if cur.Status == JOB_RUNNING {
					cur.Fail(JOB_TIMEOUT_ERROR)
				}
				break
			}
		}
		jm.Mutex.Unlock()
		if cur != nil {
			cur.Dequeue()
		}
		time.Sleep(5 * time.Second)
	}
}

func (jm *JobManager) FindById(id int) *Job {
	jm.Mutex.Lock()
	defer jm.Mutex.Unlock()
	for cur := jm.First; cur != nil; cur = cur.next {
		if cur.Id == id {
			return cur
		}
	}
	return nil
}

func (jm *JobManager) NewJob(updater JobUpdater) *Job {
	if updater == nil {
		updater = NopUpdater
	}
	job := &Job{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Progress:  0,
		Status:    JOB_RUNNING,
		Error:     nil,
		updater:   updater,
		manager:   nil,
		next:      nil,
	}
	jm.Mutex.Lock()
	defer jm.Mutex.Unlock()

	jm.TopId++
	job.Id = jm.TopId
	job.manager = jm
	if jm.First == nil {
		jm.First = job
		jm.Last = job
	} else {
		jm.Last.next = job
		jm.Last = job
	}
	clog.DebugXX("Creating job %d.", job.Id)
	return job
}

func (job *Job) Dequeue() bool {
	jm := job.manager

	jm.Mutex.Lock()
	defer jm.Mutex.Unlock()

	var prev **Job = &jm.First
	for cur := jm.First; cur != nil; cur = cur.next {
		if cur == job {
			clog.DebugXX("Dequeueing job %d.", job.Id)
			*prev = cur.next
			return true
		}
		prev = &cur.next
	}
	return false
}

func (job *Job) Touch() {
	job.UpdatedAt = time.Now()
}

func (job *Job) UpdateProgress(progress float32) {
	job.Touch()
	job.Progress = progress
	job.updater.Update(job)
}

func (job *Job) Fail(err error) {
	job.Touch()
	job.Error = err
	job.Status = JOB_ERROR
	job.updater.Update(job)
}

func (job *Job) Success() {
	job.Touch()
	job.Status = JOB_SUCCESS
	job.updater.Update(job)
}
