package checkbot

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os/signal"
	"sort"
	"strings"

	"github.com/MathieuTurcotte/go-trie/gtrie"
	"github.com/felicson/checkbot/internal/flags"

	//_ "net/http/pprof"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"
)

var (
	ErrWrongDateFormat = errors.New("wrong date format")
	ErrWrongLogLine    = errors.New("wrong log string")
	validBots          = map[string]struct{}{
		"googlebot.com": {},
		"google.com":    {},
		"yandex.ru":     {},
		"yandex.net":    {},
		"yandex.com":    {},
		"mail.ru":       {},
		"msn.com":       {},
		"sputnik.ru":    {},
	}

	whitePath = []string{
		"/captcha",
		"/ajax/",
		"/apple-touch-icon",
		"/board/context_if",
		"/context_if",
		"/favicon.ico",
		"/product_upload",
		"/profile/board/group/",
		"/profile/trade",
		"/promotion/ad/",
		"/simple_captcha",
		"/st-", "/st.css",
		"/stat/",
		"/trading_upload",
		"/user/ajax_price",
		"/user/edit",
		"/user/get_domain",
		"/user/get_reg_city",
		"/user/stock_ajax"}
)

type Firewaller interface {
	AddIP(string) error
	RemoveIP(string) error
}

type LogRecord struct {
	IP         net.IP
	Path       string
	Date       time.Time
	Bytes      uint64
	StatusCode int
}

type LogsList []*LogFile

type User struct {
	IP        string
	Hits      uint16
	WhiteHits uint16
	average   float32
	White     bool
	Banned    bool
	Checked   bool
	Bytes     uint64
}

type Users struct {
	Row        map[string]*User
	mu         sync.Mutex
	today      time.Time
	trie       *gtrie.Node
	firewaller Firewaller
	IPChan     chan string
	wlist      flags.Whitelist
}

func NewUsers(firewaller Firewaller, wl flags.Whitelist) (*Users, error) {

	sort.Strings(whitePath)
	trie, err := gtrie.Create(whitePath)

	if err != nil {
		return nil, err
	}

	u := Users{
		Row:        make(map[string]*User),
		today:      today(),
		trie:       trie,
		firewaller: firewaller,
		IPChan:     make(chan string, 10),
		wlist:      wl,
	}

	go u.loop()

	return &u, nil
}

//IsWhitePath check is received path are contains in white list
func (users *Users) IsWhitePath(path string) bool {

	tmpTrie := users.trie

	for _, l := range path {
		curr := tmpTrie.GetChild(l)
		if curr == nil {
			break
		}
		tmpTrie = curr
		if curr.Terminal {
			return true
		}
	}
	return false
}

func (users *Users) Push(item *User) {

	users.mu.Lock()
	defer users.mu.Unlock()
	users.Row[item.IP] = item
}

func (users *Users) Get(ip string) (*User, bool) {

	item, ok := users.Row[ip]
	return item, ok
}

//Truncate clear all existing data
func (users *Users) Truncate() {

	users.mu.Lock()
	defer users.mu.Unlock()
	users.Row = make(map[string]*User)
	users.today = today()
}

func (users *Users) Lookup(user *User) {

	dns, err := net.LookupAddr(user.IP)

	user.Checked = true

	if _, ok := users.wlist[user.IP]; ok {
		user.White = true
		return
	}

	if err != nil {
		user.Banned = true
		log.Printf("Banned not resolved %s", user.IP)
		users.IPChan <- user.IP
		return
	}

	if isBotValid(dns[0]) {
		log.Printf("White IP detected: %s\n", user.IP)
		user.White = true
		return
	}
	log.Printf("IP %s has been blocked\n", user.IP)
	user.Banned = true
	users.IPChan <- user.IP
}

func (users *Users) HandleEvent(line []byte) error {

	logRecord, err := ExtractIP(line)
	if err != nil {
		return fmt.Errorf("on extract ip error: %v\n", err)
	}
	ip := logRecord.IP.String()

	if !logRecord.Date.Equal(users.today) || logRecord.StatusCode == 302 {
		return nil
	}

	if item, ok := users.Get(ip); ok {

		if users.IsWhitePath(logRecord.Path) {
			item.HitsBytesIncrement(logRecord.Bytes, true)
			return nil
		}

		item.HitsBytesIncrement(logRecord.Bytes, false)

		if item.NotVerified() && item.isExceededLimit() {
			users.Lookup(item)
		}
		return nil
	}
	users.Push(NewUser(ip, logRecord.Bytes))
	return nil
}

func (users *Users) loop() {

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case ip := <-users.IPChan:
			if err := users.firewaller.AddIP(ip); err != nil {
				log.Printf("on block IP: %v\n", err)
			}
		case s := <-sig:
			switch s {
			case syscall.SIGHUP:
				users.Truncate()
				log.Println("Signal HUP received")

			default:
				log.Println("Signal KILL received")
				os.Exit(0)
			}
		}
	}
}

func NewUser(ip string, bytes uint64) *User {
	return &User{
		IP:        ip,
		Hits:      1,
		WhiteHits: 1,
		Bytes:     bytes,
	}
}

func (user *User) HitsBytesIncrement(bytes uint64, white bool) {

	user.Bytes += bytes
	if white {
		user.WhiteHits += 1
		return
	}
	user.Hits += 1
}

func (user *User) isExceededLimit() bool {

	return int16(user.Hits-user.WhiteHits) > 25
}

func (user *User) NotVerified() bool {

	return (!user.White && !user.Banned && user.Checked == false) || (!user.Checked && user.isExceededLimit())
}

func ExtractIP(row []byte) (LogRecord, error) {
	a := splitN(row)
	if len(a[3]) < 13 {
		return LogRecord{}, ErrWrongDateFormat
	}
	date, err := time.Parse("02/Jan/2006", a[3][1:12])
	if err != nil {
		return LogRecord{}, fmt.Errorf("on time parse: %v", err)
	}
	code, _ := strconv.Atoi(a[8])
	downloaded, _ := strconv.ParseUint(a[9], 10, 64)
	return LogRecord{
		IP:         net.ParseIP(a[0]),
		Path:       a[6],
		Date:       date,
		Bytes:      downloaded,
		StatusCode: code,
	}, nil
}

func today() time.Time {
	return time.Now().UTC().Truncate(24 * time.Hour)
}

func timeTrack(start time.Time, name string) {
	fmt.Printf("%s name took %s \n", name, (time.Since(start) / time.Nanosecond))
}

func isBotValid(addr string) bool {

	addrlen := len(addr) - 1
	flag, index := 0, 0

	for i := addrlen; i > 0; i-- {

		if addr[i] == '.' {
			flag++
		}
		if flag == 3 {
			index = i + 1
			break
		}
	}

	if addrlen > 5 {
		domain := addr[index:addrlen]
		if _, ok := validBots[domain]; ok {
			return true
		}
	}
	return false
}

func splitN(data []byte) []string {
	s := string(data)
	var a = make([]string, 11)
	i := 0
	sep := " "
	for i < 11 {
		m := strings.Index(s, sep)
		if m < 0 {
			break
		}
		a[i] = s[:m]
		s = s[m+len(sep):]
		i++
	}
	return a
}
