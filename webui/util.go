package webui

import (
	"encoding/json"
	"fmt"
	"github.com/omzlo/clog"
	"github.com/omzlo/nocanc/helper"
	"net/http"
	"path"
	"strings"
)

/* JSON STUFF */

func JsonSendWithStatus(w http.ResponseWriter, req *http.Request, content interface{}, status int) {
	var s []byte
	var err error

	if content != nil {
		s, err = json.MarshalIndent(content, "", "  ")
		if err != nil {
			s = []byte(fmt.Sprintf(`{ "status": 500, "error": "internal error", "information": %q }`, err))
			status = 500
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if content != nil {
		s = append(s, '\n')
		w.Write(s)
	}
}

func JsonSend(w http.ResponseWriter, req *http.Request, content interface{}) {
	JsonSendWithStatus(w, req, content, 200)
}

func ErrorSend(w http.ResponseWriter, req *http.Request, e *helper.ExtendedError) {
	clog.Warning("Request to %s returns %d %s: %s", req.URL.Path, e.Status, e.ErrorMessage, e.Information)
	JsonSendWithStatus(w, req, e, e.Status)
}

/* HTTP MUX STUFF */

type Parameters struct {
	Value map[string]string
}

func (p *Parameters) String() string {
	var result string

	for key, value := range p.Value {
		result += fmt.Sprintf("%s:%s,", key, value)
	}
	return strings.TrimRight(result, ",")
}

func NewParameters() *Parameters {
	return &Parameters{Value: make(map[string]string)}
}

type Handler interface {
	ServeHTTP(http.ResponseWriter, *http.Request, *Parameters)
}

type HandlerDescriptor struct {
	Pattern string
	Handler
}

type HandlerFunc func(http.ResponseWriter, *http.Request, *Parameters)

func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request, params *Parameters) {
	f(w, r, params)
}

type GoHandler struct {
	handler http.Handler
}

func SimpleHandler(h http.Handler) GoHandler {
	return GoHandler{h}
}

func (g GoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, params *Parameters) {
	g.handler.ServeHTTP(w, r)
}

type ServeMux struct {
	Patterns []HandlerDescriptor
}

func NewServeMux() *ServeMux {
	return &ServeMux{Patterns: make([]HandlerDescriptor, 0, 8)}
}

func (mux *ServeMux) Handle(pattern string, handler Handler) {
	for _, hd := range mux.Patterns {
		if hd.Pattern == pattern {
			panic(fmt.Sprintf("Duplicate pattern <%s> in mux.", pattern))
		}
	}

	mux.Patterns = append(mux.Patterns, HandlerDescriptor{pattern, handler})
}

func (mux *ServeMux) HandleFunc(pattern string, handler_func func(http.ResponseWriter, *http.Request, *Parameters)) {
	if handler_func == nil {
		panic("nil handler for " + pattern)
	}
	mux.Handle(pattern, HandlerFunc(handler_func))
}

func pat_match(req *http.Request, pat string) (bool, *Parameters) {
	path := path.Clean(req.URL.Path)

	params := NewParameters()

	src_parts := strings.Split(strings.TrimRight(path, "/"), "/")
	src_parts[0] = req.Method

	pat_parts := strings.Split(strings.TrimRight(pat, "/"), "/")
	pat_parts[0] = strings.Trim(pat_parts[0], " ")

	for i, pat_item := range pat_parts {
		if len(pat_item) == 0 {
			return false, nil
		}
		if pat_item == "*" {
			return true, params
		}
		if i >= len(src_parts) {
			return false, nil
		}
		if pat_item[0] == ':' {
			params.Value[pat_item[1:]] = src_parts[i]
		} else if pat_item != src_parts[i] {
			return false, nil
		}
	}
	if len(src_parts) != len(pat_parts) {
		return false, nil
	}

	query := req.URL.Query()
	for k, v := range query {
		if _, ok := params.Value[k]; !ok {
			params.Value[k] = strings.Join(v, ",")
		} else {
			clog.DebugXX("Query string contains parameter '%s' already included in pattern '%s'.", k, pat)
			return false, nil
		}
	}
	return true, params
}

func (mux *ServeMux) Handler(r *http.Request) (h Handler, pattern string, params *Parameters) {
	for _, hd := range mux.Patterns {
		ok, params := pat_match(r, hd.Pattern)
		if ok {
			return hd.Handler, hd.Pattern, params
		}
	}
	return nil, "", nil
}

type LogResponseWriter struct {
	Origin     http.ResponseWriter
	StatusCode int
	TotalBytes int
}

func NewLogResponseWriter(origin http.ResponseWriter) *LogResponseWriter {
	return &LogResponseWriter{Origin: origin, StatusCode: 0, TotalBytes: 0}
}

func (l *LogResponseWriter) Header() http.Header {
	return l.Origin.Header()
}

func (l *LogResponseWriter) Write(b []byte) (int, error) {
	r, e := l.Origin.Write(b)
	l.TotalBytes += r
	return r, e
}

func (l *LogResponseWriter) WriteHeader(statusCode int) {
	l.StatusCode = statusCode
	l.Origin.WriteHeader(statusCode)
}

func (mux *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler, _, params := mux.Handler(r)
	if handler == nil {
		ErrorSend(w, r, helper.NotFound("No handler"))
		return
	}
	logger := NewLogResponseWriter(w)
	handler.ServeHTTP(logger, r, params)
	clog.Info("%s \"%s %s\" [%s] %d %d", r.RemoteAddr, r.Method, r.URL.Path, params, logger.StatusCode, logger.TotalBytes)

}
