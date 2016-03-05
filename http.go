package main

import (
	"bufio"
	"fmt"
	//	"github.com/likexian/whois-go"
	"html/template"
	"net"
	"net/http"
	"os"
	"os/exec"
	"pid"
	"strings"
)

var (
	tmpl     *template.Template
	tmpl_err error
)

const HTML = `<table cellpadding="10" border=1>
				<tr>
					<th>#</th>
					<th>IP</th>
					<th>Hits</th>
					<th>Valid hits</th>
					<th>Checked</th>
					<th>Result</th>
					<th>Down</th>
					<th colspan="3">Actions</th>
			  </tr>{{with .}}{{range $index,$elem := .}}<tr>
				  <td>{{$index}}</td><td>{{.IP}}</td>
				  <td>{{.Hits}}</td>
				  <td>{{.White_hits}}</td>
				  <td>{{if .Checked}}YES{{end}}</td>
				  <td>{{if .White}}<span style="background-color:#DFF0D8;">GOOD</span>{{end}}{{if .Banned}}<span style="background-color:#F2DEDE;">BAD</span>{{end}}</td>
				  <td>{{.Bytes|mgb}}</td>
				  <td><a href="/info/ip?find={{.IP}}">view log</a></td>
				  {{if .Banned}}
					<td style="background-color:#F2DEDE;"><a href="/info/ip/ban?ip={{.IP}}&action=unban">unban</a></td>
				  {{else}}
					<td style="background-color:#DFF0D8;"><a href="/info/ip/ban?ip={{.IP}}&action=ban">ban</a></td>
				  {{end}}
				  <td><a href="/info/whois?ip={{.IP}}">whois</a></td>
			  </tr>{{end}}{{end}}
			  </table>
			  {{range $index, $page := .Pages}} 
				  <a href="/info/?p={{$page}}">{{$page}}</a>
			  {{end}}
			  `

func (storage *Items) InfoHandler(w http.ResponseWriter, r *http.Request) {

	var p string

	if p = r.FormValue("p"); p == "" {
		p = "0"
	}

	//hits := func(i1, i2 *Item) bool { return i1.Hits > i2.Hits }
	bytes := func(i1, i2 *Item) bool { return i1.Bytes > i2.Bytes }

	By(bytes).Sort(storage.array)

	tmpl.Execute(w, storage.array.Offset(p))

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

func FindHandler(w http.ResponseWriter, r *http.Request) {

	var pattern string

	if pattern = r.FormValue("find"); pattern == "" {
		w.Write([]byte("Not set pattern"))
		return
	}

	for _, log := range Logs {

		file, err := os.Open(log.path)

		defer file.Close()

		if err != nil {
			http.Error(w, "Error open logfile", 500)
			return
		}

		fmt.Fprintf(w, "\n\n ######### %s #########\n\n", log.path)

		scan := bufio.NewScanner(file)
		for scan.Scan() {
			if strings.Contains(strings.ToLower(scan.Text()), strings.ToLower(pattern)) {
				fmt.Fprintln(w, scan.Text())
			}
		}
	}
}

func WhoisHandler(w http.ResponseWriter, r *http.Request) {

	var ip string

	if ip = r.FormValue("ip"); ip == "" {
		w.Write([]byte("Not set pattern"))
		return
	}
	//whois, err := whois.Whois(pattern, "whois.arin.net", "whois.pir.org")
	whois, err := exec.Command("/usr/bin/whois", ip).Output()

	if err != nil {
		http.Error(w, err.Error(), 500)
	}

	w.Write(whois)

}

func init() {

	fmap := map[string]interface{}{

		"mgb": func(bytes uint64) string {

			return fmt.Sprintf("%.3f Mb", float64(bytes)/(1024*1024))
		},
	}

	tmpl, tmpl_err = template.New("test").Funcs(fmap).Parse(HTML)
}

func Run(storage *Items) {

	//	const SOCKET = "/tmp/checkbot.sock"

	//	if _, err := os.Stat(SOCKET); err == nil {
	//
	//		logchan <- "Remove old socket file"
	//		os.Remove(SOCKET)
	//	}

	//	listener, err := net.Listen("unix", SOCKET)
	listener, err := net.Listen("tcp", ":9000")

	if err != nil {
		panic(err)
	}

	http.HandleFunc("/info/", storage.InfoHandler)
	http.HandleFunc("/info/ip", FindHandler)
	http.HandleFunc("/info/ip/ban", storage.banHandler)
	http.HandleFunc("/info/whois", WhoisHandler)

	//	err = os.Chmod(SOCKET, 0777)
	//
	//	if err != nil {
	//		panic(err)
	//	}

	pid.CreatePid()

	fmt.Println(http.Serve(listener, nil))
}
