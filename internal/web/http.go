package web

import (
	"errors"
	"html/template"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/felicson/checkbot"
	"github.com/felicson/checkbot/internal/producer"
	"github.com/felicson/checkbot/internal/web/view"
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
	users      *checkbot.Users
	searcher   producer.Searcher
	firewaller checkbot.Firewaller
	listener   net.Listener
	view       *view.View
}

func (s *Server) infoHandler(w http.ResponseWriter, r *http.Request) {

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
	s.view.Render(w, "index", bots)
}

func (s *Server) banHandler(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()

	if err != nil {
		io.WriteString(w, "Wrong input data")
		return
	}

	ip := r.FormValue("ip")

	action := r.FormValue("action")

	var firewallAction func(string) error
	var itemValue bool

	switch action {

	case "ban":
		firewallAction = s.firewaller.AddIP
		itemValue = true

	case "unban":
		firewallAction = s.firewaller.RemoveIP

	default:
		io.WriteString(w, "Wrong input data")
		return
	}

	if user, ok := s.users.Get(ip); ok {
		user.Banned = itemValue
		_ = firewallAction(user.IP)
		http.Redirect(w, r, "/info/", 302)
		return

	}
	io.WriteString(w, "Wrong input data")
}

//findHandler find pattern in log files. Allowed any value
func (s *Server) findHandler(w http.ResponseWriter, r *http.Request) {

	var pattern string

	if pattern = r.FormValue("find"); pattern == "" {
		http.Error(w, "Pattern not set", 500)
		return
	}
	matches, err := s.searcher.SearchByPattern(pattern)
	if err != nil {
		http.Error(w, "err on search by pattern", 500)
		return
	}

	data := struct {
		Pattern string
		Matches producer.Matchers
	}{
		pattern, matches,
	}
	s.view.Render(w, "ipinfo", data)
}

//whoisHandler get whois info by ip address
func (s *Server) whoisHandler(w http.ResponseWriter, r *http.Request) {

	var ip string

	if ip = r.FormValue("ip"); ip == "" {
		io.WriteString(w, "Not set pattern")
		return
	}
	if net.ParseIP(ip) == nil {
		io.WriteString(w, "Wrong IP address was received")
		return
	}
	whois, err := whois.Lookup(ip)

	if err != nil {
		http.Error(w, err.Error(), 500)
	}
	user, _ := s.users.Get(ip)

	data := struct {
		Item  *checkbot.User
		Whois string
	}{user, string(whois.Data)}

	s.view.Render(w, "whois", data)

}

func init() {

	apool = sync.Pool{
		New: func() interface{} { return make([]*checkbot.User, 10) },
	}
}

func NewServer(users *checkbot.Users, searcher producer.Searcher, firewaller checkbot.Firewaller) *Server {

	return &Server{
		users:      users,
		searcher:   searcher,
		firewaller: firewaller,
	}
}

func (s *Server) Stop() {
	os.Remove(SOCKET)
	s.listener.Close()
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
	s.listener = listener

	if s.listener.Addr().Network() == "unix" {
		if err = os.Chmod(SOCKET, 0777); err != nil {
			return err
		}
	}

	v, err := view.NewView()
	if err != nil {
		return err
	}
	s.view = v

	http.HandleFunc("/info/", s.infoHandler)
	http.HandleFunc("/info/ip", s.findHandler)
	http.HandleFunc("/info/ip/ban", s.banHandler)
	http.HandleFunc("/info/whois", s.whoisHandler)
	http.HandleFunc("/info/processes", s.processHandler)

	http.HandleFunc("/info/assets/", s.view.AssetHandler)

	go func() {
		if err := http.Serve(s.listener, nil); err != nil {
			errChan <- err
		}
	}()
	select {
	case err := <-errChan:
		return err
	case <-time.After(200 * time.Millisecond):
		return nil
	}
}
