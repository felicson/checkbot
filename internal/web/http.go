package web

import (
	"bufio"
	"errors"
	"fmt"
	"html/template"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/felicson/checkbot"
	"github.com/martinolsen/go-whois"
)

const (
	DELIM  = 50
	SOCKET = "/tmp/checkbot.sock"
)

var (
	tmpl     *template.Template
	tmpl_err error

	apool sync.Pool
)

type By func(i1, i2 *checkbot.User) bool

func (by By) Sort(bots []*checkbot.User) ItemsList {

	is := ItemsList{Items: bots, by: by, length: len(bots)}
	sort.Sort(is)
	return is
}

type ItemsList struct {
	Items  []*checkbot.User
	by     func(i1, i2 *checkbot.User) bool
	length int
}

func (item ItemsList) Len() int { return len(item.Items) }

func (item ItemsList) Swap(i, j int) { item.Items[i], item.Items[j] = item.Items[j], item.Items[i] }

func (item ItemsList) Less(i, j int) bool {

	return item.by(item.Items[i], item.Items[j])

}

func (items *ItemsList) Pages() []int {

	var pages []int
	pages_num := math.Ceil(float64(items.length) / float64(DELIM))
	if pages_num > 0 {

		pages = make([]int, int(pages_num))
		for i := range pages {
			pages[i] = i + 1
		}
	}
	return pages

}
func (i *ItemsList) Offset(start string) (*ItemsList, error) {

	offset, err := strconv.Atoi(start)
	if err != nil {
		offset = 0
	} else {

		if offset > 0 {
			offset -= 1
		}
	}
	end := (offset * DELIM) + DELIM

	if len(i.Items) >= end {
		i.Items = i.Items[offset*DELIM : end]
		return i, nil
	}

	if offset*DELIM > len(i.Items) {
		return nil, errors.New("Wrong offset")
	}
	i.Items = i.Items[offset*DELIM:]
	return i, nil
}

type Server struct {
	users    *checkbot.Users
	logs     []checkbot.LogFile
	firewall checkbot.Firewaller
}

func (s *Server) InfoHandler(w http.ResponseWriter, r *http.Request) {

	var p string

	if p = r.FormValue("p"); p == "" {
		p = "0"
	}

	var bySort By

	bySort = func(i1, i2 *checkbot.User) bool { return i1.Hits > i2.Hits }

	by := r.FormValue("sort")

	switch by {

	case "bytes":
		bySort = func(i1, i2 *checkbot.User) bool { return i1.Bytes > i2.Bytes }

	case "valid":
		bySort = func(i1, i2 *checkbot.User) bool { return i1.WhiteHits > i2.WhiteHits }

	}
	array := apool.Get().([]*checkbot.User)
	storageLen := len(s.users.Row)

	if len(array) < storageLen {
		array = make([]*checkbot.User, storageLen)
	}

	defer apool.Put(array)
	i := 0
	for _, v := range s.users.Row {
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

func (s *Server) banHandler(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()

	if err != nil {
		w.Write([]byte("Wrong input data"))
		return
	}

	ip := r.FormValue("ip")

	action := r.FormValue("action")

	var firewallAction func(string)
	var itemValue bool

	switch action {

	case "ban":
		firewallAction = s.firewall.AddIP
		itemValue = true

	case "unban":
		firewallAction = s.firewall.RemoveIP
		itemValue = false

	default:
		w.Write([]byte("Wrong input data"))
		return
	}

	if user, ok := s.users.Get(ip); ok {
		user.Banned = itemValue
		firewallAction(user.IP)
		http.Redirect(w, r, "/info/", 302)
		return

	}
	w.Write([]byte("Wrong input data"))

}

//FindHandler find pattern in log files. Allowed any value
func (s *Server) FindHandler(w http.ResponseWriter, r *http.Request) {

	var pattern string

	if pattern = r.FormValue("find"); pattern == "" {
		http.Error(w, "Pattern not set", 500)
		return
	}

	matches := make(map[string][]string)

	for _, log := range s.logs {

		func(log checkbot.LogFile) {
			file, err := os.Open(log.Path)

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
			matches[log.Path] = tmp

		}(log)
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
func (s *Server) WhoisHandler(w http.ResponseWriter, r *http.Request) {

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
	storageIP, _ := s.users.Get(ip)

	data := struct {
		Item  *checkbot.User
		Whois string
	}{storageIP, string(whois.Data)}

	renderTemplate(w, "whois", data)

}

func init() {

	apool = sync.Pool{
		New: func() interface{} { return make([]*checkbot.User, 10) },
	}
}

func NewServer(users *checkbot.Users) *Server {
	return &Server{
		users: users,
	}
}

func (s *Server) Stop() {
	os.Remove(SOCKET)
}

//Run webserver start
func (s *Server) Run() error {

	var err error
	errChan := make(chan error)

	if _, err = os.Stat(SOCKET); err == nil {

		log.Println("Remove old socket file")
		os.Remove(SOCKET)
	}

	//listener, err := net.Listen("unix", SOCKET)
	listener, err := net.Listen("tcp4", "0.0.0.0:9001")

	if err != nil {
		return err
	}
	//defer listener.Close()

	//if err = os.Chmod(SOCKET, 0777); err != nil {
	//	return err
	//}
	initTempaltes()

	http.HandleFunc("/info/", s.InfoHandler)
	http.HandleFunc("/info/ip", s.FindHandler)
	http.HandleFunc("/info/ip/ban", s.banHandler)
	http.HandleFunc("/info/whois", s.WhoisHandler)
	http.HandleFunc("/info/assets/app.css", assetHandler)
	http.HandleFunc("/info/processes", processHandler)

	go func() {
		if err := http.Serve(listener, nil); err != nil {
			fmt.Println(err)
			errChan <- err
		}
	}()
	select {
	case err := <-errChan:
		return err
	case <-time.After(2 * time.Second):
		return nil
	}
}
