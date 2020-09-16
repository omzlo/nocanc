package helper

import (
	"encoding/json"
	"fmt"
	"github.com/omzlo/nocand/socket"
	"net/http"
)

/* ERROR STUFF */

type ExtendedError struct {
	Status       int    `json:"status"`
	ErrorMessage string `json:"error"`
	Information  string `json:"information,omitempty"`
}

func NewExtendedError(status int, err string, info interface{}) *ExtendedError {
	switch v := info.(type) {
	case string:
		return &ExtendedError{status, err, v}
	case fmt.Stringer:
		return &ExtendedError{status, err, v.String()}
	case error:
		return &ExtendedError{status, err, v.Error()}
	default:
		return &ExtendedError{status, err, ""}
	}
}

func ExtendError(err error) *ExtendedError {
	if xerr, ok := err.(*ExtendedError); ok {
		return xerr
	}

	switch err {
	case nil:
		return nil
	case socket.ErrorServerAckBadRequest:
		return BadRequest(err)
	case socket.ErrorServerAckUnauthorized:
		return Unauthorized(err)
	case socket.ErrorServerAckNotFound:
		return NotFound(err)
	case socket.ErrorServerAckGeneralFailure:
		return InternalServerError(err)
	}
	return ServiceUnavailable(err)
}

func (e *ExtendedError) WithInformation(info string) *ExtendedError {
	e.Information = info
	return e
}

func (e *ExtendedError) Error() string {
	if e.Information != "" {
		return fmt.Sprintf("%s: %s", e.ErrorMessage, e.Information)
	}
	return fmt.Sprintf("%s", e.ErrorMessage)
}

func (e *ExtendedError) String() string {
	json, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
	}
	return string(json)
}

func ServiceUnavailable(info interface{}) *ExtendedError {
	return NewExtendedError(http.StatusServiceUnavailable, "nocand server error", info)
}

func NotFound(info interface{}) *ExtendedError {
	return NewExtendedError(http.StatusNotFound, "not found", info)
}

func BadRequest(info interface{}) *ExtendedError {
	return NewExtendedError(http.StatusBadRequest, "bad request", info)
}

func InternalServerError(info interface{}) *ExtendedError {
	return NewExtendedError(http.StatusInternalServerError, "internal server error", info)
}

func Unauthorized(info interface{}) *ExtendedError {
	return NewExtendedError(http.StatusUnauthorized, "unauthorized", info)
}
