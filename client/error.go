package client

import (
	"encoding/json"
	"fmt"
	"net/http"
)

/* ERROR STUFF */

type Error struct {
	Status      int    `json:"status"`
	Error       string `json:"error"`
	Information string `json:"information,omitempty"`
}

func NewError(status int, err string, info interface{}) *Error {
	switch v := info.(type) {
	case string:
		return &Error{status, err, v}
	case fmt.Stringer:
		return &Error{status, err, v.String()}
	case error:
		return &Error{status, err, v.Error()}
	default:
		return &Error{status, err, ""}
	}
}

func (e *Error) GoError() error {
	if e.Information != "" {
		return fmt.Errorf("%s: %s", e.Error, e.Information)
	}
	return fmt.Errorf("%s", e.Error)
}

func (e *Error) String() string {
	json, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
	}
	return string(json)
}

func ServiceUnavailable(info interface{}) *Error {
	return NewError(http.StatusServiceUnavailable, "nocand server error", info)
}

func NotFound(info interface{}) *Error {
	return NewError(http.StatusNotFound, "not found", info)
}

func BadRequest(info interface{}) *Error {
	return NewError(http.StatusBadRequest, "bad request", info)
}

func InternalServerError(info interface{}) *Error {
	return NewError(http.StatusInternalServerError, "internal server error", info)
}
