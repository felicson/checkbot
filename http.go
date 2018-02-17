package main

//go:generate go-bindata -nocompress templates/... assets/
import (
	"bufio"
	"fmt"
	"github.com/martinolsen/go-whois"
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	tmpl     *template.Template
	tmpl_err error
	t        map[string]*template.Template

	apool sync.Pool
)

func renderTemplate(w http.ResponseWriter, name string, data interface{}) error {

	tpl, ok := t[name]

	if !ok {
		return fmt.Errorf("The template %s does not exist!", name)
	}

	return tpl.ExecuteTemplate(w, "base", data)
}

func (storage *Items) InfoHandler(w http.ResponseWriter, r *http.Request) {

	var p string

	if p = r.FormValue("p"); p == "" {
		p = "0"
	}

	var bySort By

	bySort = func(i1, i2 *Item) bool { return i1.Hits > i2.Hits }

	by := r.FormValue("sort")

	switch by {

	case "bytes":
		bySort = func(i1, i2 *Item) bool { return i1.Bytes > i2.Bytes }

	case "valid":
		bySort = func(i1, i2 *Item) bool { return i1.WhiteHits > i2.WhiteHits }

	}
	array := apool.Get().([]*Item)
	storageLen := len(storage.row)

	if len(array) < storageLen {
		array = make([]*Item, storageLen)
	}

	defer apool.Put(array)
	i := 0
	for _, v := range storage.row {
		array[i] = v
		i++
	}
	data := By(bySort).Sort(array)
	bots, err := data.Offset(p)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	renderTemplate(w, "index", bots)

}

func (storage *Items) banHandler(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()

	if err != nil {
		w.Write([]byte("Wrong input data"))
		return
	}

	ip := r.FormValue("ip")

	action := r.FormValue("action")

	var actionValue string
	var itemValue bool

	switch action {

	case "ban":

		actionValue = "add"
		itemValue = true

	case "unban":

		actionValue = "del"
		itemValue = false

	default:
		w.Write([]byte("Wrong input data"))
		return
	}

	if item, ok := storage.Get(ip); ok {

		item.Banned = itemValue
		execCommand(fmt.Sprintf("sudo /sbin/ipset %s blacklist %s", actionValue, ip))
		http.Redirect(w, r, "/info/", 302)
		return

	}
	w.Write([]byte("Wrong input data"))

}

//FindHandler find pattern in log files. Allowed any value
func FindHandler(w http.ResponseWriter, r *http.Request) {

	var pattern string

	if pattern = r.FormValue("find"); pattern == "" {
		http.Error(w, "Pattern not set", 500)
		return
	}

	matches := make(map[string][]string)

	for _, log := range Logs {

		file, err := os.Open(log.path)

		defer file.Close()

		if err != nil {
			http.Error(w, "Error open logfile", 500)
			return
		}
		scan := bufio.NewScanner(file)

		var tmp []string
		for scan.Scan() {
			line := scan.Text()
			if strings.Contains(line, pattern) {
				tmp = append(tmp, line)
			}
		}
		matches[log.path] = tmp
	}
	data := struct {
		Pattern string
		Matches map[string][]string
	}{
		pattern, matches,
	}
	renderTemplate(w, "ipinfo", data)
}

//WhoisHandler get whois info by ip address
func (storage *Items) WhoisHandler(w http.ResponseWriter, r *http.Request) {

	var ip string

	if ip = r.FormValue("ip"); ip == "" {
		w.Write([]byte("Not set pattern"))
		return
	}
	if net.ParseIP(ip) == nil {
		w.Write([]byte("Wrong IP address was received"))
		return
	}
	whois, err := whois.Lookup(ip)

	if err != nil {
		http.Error(w, err.Error(), 500)
	}
	storageIP, _ := storage.Get(ip)

	data := struct {
		Item  *Item
		Whois string
	}{storageIP, string(whois.Data)}

	renderTemplate(w, "whois", data)

}
func assetHandler(rw http.ResponseWriter, r *http.Request) {

	css, err := Asset("assets/app.css")

	if err != nil {
		http.Error(rw, "wrong css", 500)
	}
	rw.Header().Set("Content-type", "text/css")
	rw.Write(css)
}

func init() {

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
	}

	templates := AssetNames()

	t = make(map[string]*template.Template, len(templates))

	var tb *template.Template

	for _, tpl := range templates {

		if strings.HasPrefix(tpl, "templates/includes") {

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

		if strings.HasPrefix(tpl, "templates/includes") || strings.HasPrefix(tpl, "assets") {
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

	apool = sync.Pool{
		New: func() interface{} { return make([]*Item, 10) },
	}
}

//Run webserver start
func Run(storage *Items) {

	var err error
	const SOCKET = "/tmp/checkbot.sock"

	if _, err = os.Stat(SOCKET); err == nil {

		logchan <- "Remove old socket file"
		os.Remove(SOCKET)
	}

	listener, err := net.Listen("unix", SOCKET)
	//listener, err := net.Listen("tcp", ":9000")

	if err != nil {
		panic(err)
	}

	err = os.Chmod(SOCKET, 0777)

	if err != nil {
		listener.Close()
		panic(err)
	}

	http.HandleFunc("/info/", storage.InfoHandler)
	http.HandleFunc("/info/ip", FindHandler)
	http.HandleFunc("/info/ip/ban", storage.banHandler)
	http.HandleFunc("/info/whois", storage.WhoisHandler)
	http.HandleFunc("/info/assets/app.css", assetHandler)

	fmt.Println(http.Serve(listener, nil))
}
