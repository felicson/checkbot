package checkbot

import (
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
	validBots = map[string]struct{}{
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

	logchan chan string

	wlist Whitelist
)

const (
	SIGHUP = syscall.SIGHUP
)

type Firewaller interface {
	AddIP(string)
	RemoveIP(string)
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

func (storage *Users) IsWhitePath(path *string) bool {

	//	defer timeTrack(time.Now(),"check in white")
	var l rune

	//	storage.Lock()
	trie2 := storage.trie

	defer func() {
		storage.trie = trie2
		//		storage.Unlock()
	}()

	for _, l = range *path {

		curr := storage.trie.GetChild(l)

		if curr == nil {
			break
		}
		storage.trie = curr
		if curr.Terminal {
			return true
		}
	}
	return false
}

func (storage *Users) Push(ip string, item *User) {

	storage.Lock()
	storage.Row[ip] = item
	storage.Unlock()
}

func (storage *Users) Get(ip string) (*User, bool) {

	item, ok := storage.Row[ip]

	return item, ok
}

func (storage *Users) Reset() {

	storage.Lock()
	defer storage.Unlock()
	storage.Row = make(map[string]*User)
	storage.today = today()
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

func ExtractIP(row *string) (LogRecord, error) {
	data := strings.SplitN(*row, " ", 11)[0:10]

	if len(data) < 10 {
		return LogRecord{}, errors.New("Wrong log string")
	}

	if len(data[3]) < 13 {
		return LogRecord{}, errors.New("Wrong date format")
	}
	date, err := time.Parse("02/Jan/2006:15:04:05 +0700", data[3][1:12])
	if err != nil {
		return LogRecord{}, fmt.Errorf("on time parse: %v", err)
	}
	code, _ := strconv.Atoi(data[8])
	bytes, _ := strconv.ParseUint(data[9], 10, 64)

	return LogRecord{
		IP:         net.ParseIP(data[0]),
		Path:       data[6],
		Date:       date.Truncate(24 * time.Hour),
		Bytes:      bytes,
		StatusCode: code,
	}, nil
}

func today() time.Time {
	return time.Now().Truncate(24 * time.Hour)
}

func (u *Users) execBan() {

	for ip := range u.IPChan {
		u.firewaller.AddIP(ip)
	}
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

func NewUser(ip string, bytes uint64) *User {

	return &User{
		IP:        ip,
		Hits:      1,
		WhiteHits: 1,
		Bytes:     bytes,
	}
}

func (storage *Users) HandleEvent(line string) error {

	logRecord, err := ExtractIP(&line)
	if err != nil {
		return fmt.Errorf("on extract ip error: %v\n", err)
	}
	ip := logRecord.IP.String()

	if logRecord.Date != storage.today {
		return nil
	}

	if item, ok := storage.Get(ip); ok {

		if logRecord.StatusCode == 302 {
			return nil
		}

		if storage.IsWhitePath(&logRecord.Path) {
			item.HitsBytesIncrement(logRecord.Bytes, true)
			return nil
		}

		item.HitsBytesIncrement(logRecord.Bytes, false)

		if item.NotVerified() && item.isExceededLimit() {
			storage.Lookup(item)
		}
		return nil
	}
	storage.Push(ip, NewUser(ip, logRecord.Bytes))
	return nil
}

func (storage *Users) sigHandler(sig chan os.Signal) {

	for {
		select {
		case s := <-sig:
			switch s {
			case SIGHUP:
				storage.Reset()
				log.Println("Signal HUP received")

			default:
				log.Println("Signal KILL received")
				os.Exit(0)
			}
		}
	}
}
