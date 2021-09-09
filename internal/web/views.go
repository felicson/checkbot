package web

//go:generate go-bindata -nocompress views/... assets/
import (
	"fmt"
	"html/template"
	"net"
	"net/http"
	"path/filepath"
	"strings"
)

var (
	t map[string]*template.Template
)

func renderTemplate(w http.ResponseWriter, name string, data interface{}) error {

	tpl, ok := t[name]

	if !ok {
		return fmt.Errorf("The template %s does not exist!", name)
	}

	return tpl.ExecuteTemplate(w, "base", data)
}

func initTempaltes() {
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
	}

	templates := AssetNames()

	t = make(map[string]*template.Template, len(templates))

	var tb *template.Template

	for _, tpl := range templates {

		if strings.HasPrefix(tpl, "views/includes") {

			extoffset := strings.LastIndexByte(tpl, '.')

			data, err := Asset(tpl)
			if err != nil {
				panic(err)
			}
			name := filepath.Base(tpl[:extoffset])
			if tb == nil {
				tb = template.New(name)
			} else if tb.Name() == name {
				continue
			} else {
				tb = tb.New(name)
			}
			_, err = tb.Parse(string(data))

			if err != nil {
				panic(err)
			}

		}
	}

	for _, tpl := range templates {

		if strings.HasPrefix(tpl, "views/includes") || strings.HasPrefix(tpl, "assets") {
			continue
		}

		data, err := Asset(tpl)

		if err != nil {
			panic(err)
		}
		extoffset := strings.LastIndexByte(tpl, '.')

		name := filepath.Base(tpl[:extoffset])

		tb2, _ := tb.Clone()

		tb2.New(name).Funcs(fmap)

		_, err = tb2.Parse(string(data))

		if err != nil {
			panic(err)
		}
		t[name] = tb2
	}
}

func assetHandler(rw http.ResponseWriter, r *http.Request) {

	css, err := Asset("assets/app.css")

	if err != nil {
		http.Error(rw, "wrong css", 500)
	}
	rw.Header().Set("Content-type", "text/css")
	rw.Write(css)
}
