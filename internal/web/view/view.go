package view

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed templates/**
var views embed.FS

//go:embed assets
var assets embed.FS

var (
	ErrTemplateNotExist = errors.New("template not exist")
)

type View struct {
	t map[string]*template.Template
}

func (v *View) Render(w http.ResponseWriter, name string, data interface{}) error {

	tpl, ok := v.t[name]

	if !ok {
		return fmt.Errorf("%s - %v", name, ErrTemplateNotExist)
	}

	return tpl.ExecuteTemplate(w, "base", data)
}

func NewView() (*View, error) {
	fmap := map[string]interface{}{

		"mgb": func(bytes uint64) string {
			return fmt.Sprintf("%.3f Mb", float64(bytes)/(1024*1024))
		},
		"nl2br": func(str string) template.HTML {
			return template.HTML(strings.Replace(str, "\n", "<br />", -1))
		},
		"validIP": func(str string) bool {
			return net.ParseIP(str) != nil
		},
		"float": func(val float32) string {
			return fmt.Sprintf("%.2f", val)
		},
		"add": func(val, arg int) int {
			return val + arg + 1
		},
	}

	getName := func(tpl string) string {
		extoffset := strings.LastIndexByte(tpl, '.')
		return filepath.Base(tpl[:extoffset])
	}

	includes, err := views.ReadDir("templates/includes")
	if err != nil {
		return nil, err
	}
	pages, err := views.ReadDir("templates")
	if err != nil {
		return nil, err
	}

	t := make(map[string]*template.Template, len(includes))

	var tb *template.Template

	for _, tpl := range includes {
		data, err := views.ReadFile("templates/includes/" + tpl.Name())
		if err != nil {
			return nil, err
		}
		name := getName(tpl.Name())
		if tb == nil {
			tb = template.New(name)
		} else if tb.Name() == name {
			continue
		} else {
			tb = tb.New(name)
		}
		if _, err = tb.Parse(string(data)); err != nil {
			return nil, err
		}
	}

	for _, tpl := range pages {

		if tpl.IsDir() {
			continue
		}
		data, err := views.ReadFile("templates/" + tpl.Name())

		if err != nil {
			return nil, err
		}
		name := getName(tpl.Name())

		tb2, _ := tb.Clone()

		tb2.New(name).Funcs(fmap)

		if _, err = tb2.Parse(string(data)); err != nil {
			return nil, err
		}
		t[name] = tb2
	}
	return &View{t: t}, nil
}

func (v *View) AssetHandler(rw http.ResponseWriter, r *http.Request) {
	assetName := filepath.Base(r.URL.Path)
	ext := filepath.Ext(assetName)
	media := mime.TypeByExtension(ext)
	css, err := assets.ReadFile("assets/" + assetName)
	if err != nil {
		http.Error(rw, "wrong asset", 500)
		return
	}
	rw.Header().Set("Content-type", media)
	rw.Write(css)
}
