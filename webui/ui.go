package webui

import (
	"fmt"
	"github.com/gobuffalo/packr/v2"
	"github.com/omzlo/clog"
	"github.com/omzlo/nocanc/client"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
)

const API_PREFIX string = "/api/v1"

var (
    refresh        uint
	mux            *ServeMux = nil
	static_files   *packr.Box
	template_files *packr.Box
)

func default_handler(w http.ResponseWriter, req *http.Request, params *Parameters) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}
	fmt.Fprintf(w, "Hello world")
}

func not_found(w http.ResponseWriter, req *http.Request, params *Parameters) {
	ErrorSend(w, req, client.NotFound("Resource does not exist, check URL"))
}

type TemplateCollectionHandler struct {
	collection *template.Template
}

func NewTemplateCollectionHandler(box *packr.Box) (*TemplateCollectionHandler, error) {
	var templ *template.Template

	files := box.List()
	for _, file := range files {
		if strings.HasSuffix(file, ".template") {
			content, err := box.FindString(file)
			name := strings.TrimSuffix(filepath.Base(file), ".template")
			clog.DebugXX("Loaded template '%s'", file)
			if err == nil {
				if templ == nil {
					templ = template.New(name)
					if _, err := templ.Parse(content); err != nil {
						return nil, err
					}
				} else {
					t := templ.New(name)
					if _, err := t.Parse(content); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return &TemplateCollectionHandler{collection: templ}, nil
}

type TemplateHandler struct {
	collection *template.Template
	name       string
}

func (handler *TemplateCollectionHandler) Handle(model string) *TemplateHandler {
	if handler.collection == nil {
		panic("Cannot handle empty template for " + model)
	}
	return &TemplateHandler{handler.collection, model}
}

type Link struct {
	Href string
	Text string
}

func (handler *TemplateHandler) ServeHTTP(w http.ResponseWriter, req *http.Request, params *Parameters) {
	var links []*Link

	items := strings.Split(strings.Trim(req.URL.Path, "/"), "/")

	links = append(links, &Link{Href: "/", Text: "Home"})
	for i, item := range items {
		if item != "" {
			links = append(links, &Link{Href: strings.Join(items[:i+1], "/"), Text: item})
		}
	}

	err := handler.collection.ExecuteTemplate(w, handler.name,
		&struct {
			Breadcrumbs []*Link
			Params      map[string]string
            Refresh     uint
		}{
			links,
			params.Value,
            refresh,
		})

	if err != nil {
		ErrorSend(w, req, client.InternalServerError(err))
		return
	}
}

func Run(addr string, refresh_rate uint) error {
	if mux != nil {
		return fmt.Errorf("Webui is already running")
	}
	mux = NewServeMux()

	static_files = packr.New("static", "./assets/static")
	template_files = packr.New("templates", "./assets/templates")

	coll, err := NewTemplateCollectionHandler(template_files)
	if err != nil {
		panic(err)
	}

    refresh = refresh_rate

	mux.HandleFunc("GET /api/v1/nodes", nodes_index)
	mux.HandleFunc("GET /api/v1/nodes/:id", nodes_show)
	mux.HandleFunc("POST /api/v1/nodes/:id/upload", nodes_upload)
	mux.HandleFunc("PUT /api/v1/nodes/:id/reboot", nodes_reboot)
	mux.HandleFunc("GET /api/v1/channels", channels_index)
	mux.HandleFunc("GET /api/v1/channels/:id", channels_show)
	mux.HandleFunc("PUT /api/v1/channels/:id", channels_update)
	mux.HandleFunc("GET /api/v1/power_status", power_status_index)
	mux.HandleFunc("GET /api/v1/device_info", device_info_index)
	mux.HandleFunc("GET /api/v1/system_properties", system_properties_index)
	mux.HandleFunc("GET /api/v1/jobs/:id", jobs_show)
	mux.HandleFunc("GET /api/v1/news", news_index)
	mux.HandleFunc("GET /api/v1/*", not_found)
	mux.Handle("GET /", coll.Handle("index"))
	mux.Handle("GET /channels/:id", coll.Handle("channels_show"))
	mux.Handle("GET /nodes/:id", coll.Handle("nodes_show"))
	mux.Handle("GET /system", coll.Handle("system_show"))
	mux.Handle("GET /static/*", SimpleHandler(http.StripPrefix("/static", http.FileServer(static_files))))
	mux.HandleFunc("GET /*", default_handler)

	clog.Info("Connect to the Webui at %s", addr)
	return http.ListenAndServe(addr, mux)
}
