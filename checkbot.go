package checkbot

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"

	"github.com/MathieuTurcotte/go-trie/gtrie"
	//_ "net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
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

	whitePath = []string{"/ajax/",
		"/apple-touch-icon",
		"/board/context_if",
		"/captcha",
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

	wlist Whitelist
)

const (
	SIGHUP = syscall.SIGHUP
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
	Row map[string]*User
	sync.Mutex
	today      time.Time
	trie       *gtrie.Node
	firewaller Firewaller
	IPChan     chan string
}

func NewUsers(firewaller Firewaller, wl Whitelist) (*Users, error) {

	trie, err := gtrie.Create(whitePath)

	if err != nil {
		return nil, err
	}

	wlist = wl

	u := Users{
		Row:        make(map[string]*User),
		today:      today(),
		trie:       trie,
		firewaller: firewaller,
		IPChan:     make(chan string, 10),
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	go u.sigHandler(sigChan)
	go u.execBan()

	return &u, nil
}

func (users *Users) IsWhitePath(path *string) bool {

	//	defer timeTrack(time.Now(),"check in white")
	var l rune

	//	storage.Lock()
	trie2 := users.trie

	defer func() {
		users.trie = trie2
		//		storage.Unlock()
	}()

	for _, l = range *path {

		curr := users.trie.GetChild(l)

		if curr == nil {
			break
		}
		users.trie = curr
		if curr.Terminal {
			return true
		}
	}
	return false
}

func (users *Users) Push(ip string, item *User) {

	users.Lock()
	users.Row[ip] = item
	users.Unlock()
}

func (users *Users) Get(ip string) (*User, bool) {

	item, ok := users.Row[ip]

	return item, ok
}

func (users *Users) Reset() {

	users.Lock()
	defer users.Unlock()
	users.Row = make(map[string]*User)
	users.today = today()
	//storage.array = make([]*Item, 0)
}

func timeTrack(start time.Time, name string) {
	fmt.Printf("%s name took %s \n", name, (time.Since(start) / time.Nanosecond))
}

type Whitelist map[string]bool

func (i *Whitelist) String() string {
	return fmt.Sprint(*i)
}

func (i *Whitelist) Set(value string) error {
	if len(*i) > 0 {
		return errors.New("ignoreip flag already set")
	}
	if !strings.Contains(value, ".") {
		return errors.New("ignoreip flag has wrong value")
	}
	*i = make(Whitelist)
	for _, v := range strings.Split(value, ",") {
		(*i)[v] = true
	}
	return nil
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

func ExtractIP(row []byte) (LogRecord, error) {
	parts := bytes.SplitN(row, []byte(" "), 11)[0:10]

	if len(parts) < 10 {
		return LogRecord{}, ErrWrongLogLine
	}

	if len(parts[3]) < 13 {
		return LogRecord{}, ErrWrongDateFormat
	}
	date, err := time.Parse("02/Jan/2006", string(parts[3][1:12]))
	if err != nil {
		return LogRecord{}, fmt.Errorf("on time parse: %v", err)
	}
	code, _ := strconv.Atoi(string(parts[8]))
	downloaded, _ := strconv.ParseUint(string(parts[9]), 10, 64)

	return LogRecord{
		IP:         net.ParseIP(string(parts[0])),
		Path:       string(parts[6]),
		Date:       date,
		Bytes:      downloaded,
		StatusCode: code,
	}, nil
}

func (users *Users) Lookup(user *User) {

	dns, err := net.LookupAddr(user.IP)

	user.Checked = true

	if _, ok := wlist[user.IP]; ok {
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
		log.Printf("White ip detected: %s\n", user.IP)
		user.White = true
		return
	}
	log.Printf("IP %s has been blocked\n", user.IP)
	user.Banned = true
	users.IPChan <- user.IP
}

func (u *Users) execBan() {

	for ip := range u.IPChan {
		u.firewaller.AddIP(ip)
	}
}

func (users *Users) HandleEvent(line []byte) error {

	logRecord, err := ExtractIP(line)
	if err != nil {
		return fmt.Errorf("on extract ip error: %v\n", err)
	}
	ip := logRecord.IP.String()

	if !logRecord.Date.Equal(users.today) {
		return nil
	}

	if item, ok := users.Get(ip); ok {

		if logRecord.StatusCode == 302 {
			return nil
		}

		if users.IsWhitePath(&logRecord.Path) {
			item.HitsBytesIncrement(logRecord.Bytes, true)
			return nil
		}

		item.HitsBytesIncrement(logRecord.Bytes, false)

		if item.NotVerified() && item.isExceededLimit() {
			users.Lookup(item)
		}
		return nil
	}
	users.Push(ip, NewUser(ip, logRecord.Bytes))
	return nil
}

func (users *Users) sigHandler(sig chan os.Signal) {

	for {
		select {
		case s := <-sig:
			switch s {
			case SIGHUP:
				users.Reset()
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

func today() time.Time {
	return time.Now().UTC().Truncate(24 * time.Hour)
}
