package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"github.com/MathieuTurcotte/go-trie/gtrie"
	"log"
	"math"
	"net"
	"net/http"
	"pid"
	//_ "net/http/pprof"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	VALID = map[string]bool{
		"googlebot.com": true,
		"google.com":    true,
		"yandex.ru":     true,
		"yandex.net":    true,
		"yandex.com":    true,
		"mail.ru":       true,
		"msn.com":       true,
		"sputnik.ru":    true,
	}

	WHITE_PATH = []string{"/ajax/", "/apple-touch-icon", "/board/context_if", "/captcha", "/context_if", "/favicon.ico", "/product_upload", "/profile/board/group/", "/profile/trade", "/promotion/ad/", "/simple_captcha", "/st-", "/st.css", "/stat/", "/trading_upload", "/user/ajax_price", "/user/edit", "/user/get_domain", "/user/get_reg_city", "/user/stock_ajax"}

	Logs []*LogFile

	banlog    *log.Logger
	logfile   string
	ignoreip  string
	banlogout *os.File
	pidFile   string
	pidFileD  *pid.LockFile

	logchan chan string

	fwchan chan string

	shortMonth = map[time.Month]string{

		1:  "Jan",
		2:  "Feb",
		3:  "Mar",
		4:  "Apr",
		5:  "May",
		6:  "Jun",
		7:  "Jul",
		8:  "Aug",
		9:  "Sep",
		10: "Oct",
		11: "Nov",
		12: "Dec",
	}
	storage *Items
)

const (
	SIGHUP   = syscall.SIGHUP
	DELIM    = 50
	todayFmt = "02/Jan/2006"
)

type LogsList []*LogFile

type Item struct {
	IP        string
	Hits      uint16
	WhiteHits uint16
	average   float32
	White     bool
	Banned    bool
	Checked   bool
	Bytes     uint64
}

type Items struct {
	row map[string]*Item
	//array []*Item
	*http.HandlerFunc
	sync.Mutex
	today string
	trie  *gtrie.Node
}

type LogFile struct {
	offset int64
	file   *os.File
	path   string
}

func NewItems() *Items {

	items := &Items{today: today()}

	items.row = make(map[string]*Item)
	//items.array = make([]*Item, 0)

	trie, err := gtrie.Create(WHITE_PATH)

	if err != nil {
		panic(err)
	}

	items.trie = trie

	return items
}

func (storage *Items) IsWhitePath(path *string) bool {

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

		if curr != nil {
			storage.trie = curr
			if curr.Terminal {
				return true
			}
		} else {
			break
		}
	}
	return false
}

func (storage *Items) Push(ip string, item *Item) {

	storage.Lock()
	storage.row[ip] = item
	//storage.array = append(storage.array, item)

	storage.Unlock()
}

func (storage Items) Get(ip string) (*Item, bool) {

	item, ok := storage.row[ip]

	return item, ok
}

func (storage *Items) Reset() {

	storage.Lock()
	defer storage.Unlock()
	storage.row = make(map[string]*Item)
	storage.today = today()
	//storage.array = make([]*Item, 0)
}

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	fmt.Println(fmt.Sprintf("%s name took %s \n", name, (elapsed / time.Nanosecond)))
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

		if ok, _ := VALID[domain]; ok {

			return true
		}
	}
	return false
}

type By func(i1, i2 *Item) bool

func (by By) Sort(bots []*Item) ItemsList {

	is := ItemsList{Items: bots, by: by, length: len(bots)}
	sort.Sort(is)
	return is
}

type ItemsList struct {
	Items  []*Item
	by     func(i1, i2 *Item) bool
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
		for i, _ := range pages {
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

func ExtractIP(row *string) (string, string, string, uint64, uint32, error) {

	data := strings.SplitN(*row, " ", 11)[0:10]

	if len(data) < 10 {
		log.Printf("Wrong log string format: %s", row)
		return "", "", "", 0, 200, errors.New("Wrong log string")
	}

	if len(data[3]) < 13 {
		log.Printf("Wrong date format : %s", data[3])
		return "", "", "", 0, 200, errors.New("Wrong date format")
	}
	time := data[3][1:12]
	bytes, _ := strconv.Atoi(data[9])
	code, _ := strconv.Atoi(data[8])
	return data[0], data[6], time, uint64(bytes), uint32(code), nil
}

func today() string {

	t := time.Now()
	return t.Format(todayFmt)
}

func fileSize(file *os.File) (int64, error) {

	fstat, err_s := file.Stat()

	if err_s != nil {
		return 0, err_s
	}

	return fstat.Size(), err_s

}

func writeLog(logchan chan string, banlog *log.Logger) {

	for x := range logchan {
		banlog.Println(x)
	}
}

func logCleanup() {
	close(logchan)
	banlogout.Close()
}

func execCommand(arg string) {

	cmd := exec.Command("/bin/sh", "-c", arg)
	//cmd := exec.Command("echo", arg)

	err := cmd.Run()

	if err != nil {
		logchan <- err.Error()
	}

}

func execBan() {

	for ip := range fwchan {

		//execCommand(fmt.Sprintf("sudo /sbin/ipset add blacklist %s", ip))
		execCommand(fmt.Sprintf("echo %s", ip))
	}
}

func (log *LogFile) SetOffset() {

	log.offset, _ = fileSize(log.file)
}

func (log *LogFile) Seek() bool {

	fsize, _ := fileSize(log.file)

	if fsize == log.offset {
		log.file.Close()
		return false
	}

	if fsize < log.offset {

		log.offset = 0
		//fmt.Println("Reset offset")
	}

	_, err := log.file.Seek(log.offset, 0)

	if err != nil {
		panic(err)
	}
	return true
}

func (ip *Item) Lookup(ip_addr string) {

	dns, err := net.LookupAddr(ip_addr)

	ip.Checked = true

	if ip_addr == ignoreip {
		ip.White = true
		return
	}

	if err != nil {
		ip.Banned = true
		//		fmt.Println("BANNED NOT resolved",ip.IP)
		logchan <- fmt.Sprintf("BANNED NOT RESOLVED %s", ip.IP)
		fwchan <- ip.IP
		return
	}

	if isBotValid(dns[0]) {

		logchan <- fmt.Sprintf("WHITE %s", ip.IP)
		ip.White = true
		return
	}
	logchan <- fmt.Sprintf("BANNED %s", ip.IP)
	fwchan <- ip.IP
	ip.Banned = true
}

func (ip *Item) IsWhite() bool {
	return ip.White
}
func (ip *Item) IsBanned() bool {

	return ip.Banned
}
func (ip *Item) HitsBytesIncrement(bytes uint64, white bool) {

	if white {
		ip.WhiteHits += 1
	} else {
		ip.Hits += 1
	}
	ip.Bytes += bytes
}

func (ip *Item) DiffHits() int16 {

	return int16(ip.Hits - ip.WhiteHits)
}

func (ip *Item) NotVerified() bool {

	return (!ip.IsWhite() && !ip.IsBanned() && ip.Checked == false) || (!ip.Checked && ip.DiffHits() > 25)
}

func NewItem(ip string, bytes uint64) *Item {

	return &Item{ip, 1, 1, 0, false, false, false, bytes}
}

func analyzer(line *string) {

	ip, path, date, bytes, code, err := ExtractIP(line)
	if err != nil {
		return
	}

	if date == storage.today {

		if item, ok := storage.Get(ip); ok {

			if code == 302 {
				return
			}

			if storage.IsWhitePath(&path) {

				item.HitsBytesIncrement(bytes, true)
				return
			}

			item.HitsBytesIncrement(bytes, false)

			if item.NotVerified() && item.DiffHits() > 25 {
				item.Lookup(ip)
			}
		} else {
			storage.Push(ip, NewItem(ip, bytes))
		}
	}
}

func logsReader(out chan string) {

	for _, log := range Logs {

		var err error

		log.file, err = os.Open(log.path)

		if err != nil {
			panic(err)
		}

		if !log.Seek() {
			continue
		}

		log.SetOffset()

		scann := bufio.NewScanner(log.file)

		for scann.Scan() {
			line := scann.Text()
			analyzer(&line)
		}

		log.file.Close()
	}
}

func SigHandler(sig os.Signal) {

	switch sig {

	case SIGHUP:
		storage.Reset()
		//execCommand("sudo /sbin/ipset flush blacklist")
		logchan <- "SIGNAL HUP RECEIVE"

	default:
		logchan <- "SIGNAL KILL RECEIVE"
		logCleanup()
		os.Remove("/tmp/checkbot.sock")
		pidFileD.Remove()
		os.Exit(0)
	}
}

func mainLoop(sig chan os.Signal, ticker *time.Ticker, out chan string) {

	for {
		select {

		case s := <-sig:
			SigHandler(s)

		case <-ticker.C:
			logsReader(out)
		}
	}
}

func init() {

	var loglist string

	flag.StringVar(&loglist, "loglist", "/home/felicson/loglist.conf", "loglist=/path/loglist.conf")
	flag.StringVar(&logfile, "logfile", "/home/felicson/checkbot.log", "logfile=/path/loglist.conf")
	flag.StringVar(&ignoreip, "ignoreip", "1.2.3.4", "ignoreip=1.2.3.4")
	flag.StringVar(&pidFile, "pidfile", "/var/run/go/checkbot.pid", "pidfile=/var/run/go/progname.pid")

	flag.Parse()

	banlogout, err := os.OpenFile(logfile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0664)

	if err != nil {
		panic("Cant open logfile")
	}

	logchan = make(chan string, 10)
	fwchan = make(chan string, 10)

	banlog = log.New(banlogout, "checkbot: ", log.LstdFlags)

	if err != nil {
		panic(err)
	}

	go writeLog(logchan, banlog)

	file, err := os.Open(loglist)

	defer file.Close()

	reader := bufio.NewScanner(file)

	//Logs = LogsList{}

	for reader.Scan() {
		Logs = append(Logs, &LogFile{0, &os.File{}, reader.Text()})

	}

	go execBan()
}

func main() {

	if flag.NFlag() < 2 {
		flag.Usage()
		os.Exit(0)
	}

	ncpu := runtime.NumCPU()

	runtime.GOMAXPROCS(ncpu)

	var err error
	pidFileD, err = pid.CreatePidFile(pidFile, 0644)

	defer pidFileD.Remove()

	if err != nil {
		panic(err)
	}

	storage = NewItems()

	ticker := time.NewTicker(2 * time.Second)

	sig := make(chan os.Signal, 1)

	signal.Notify(sig, SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	defer ticker.Stop()

	line := make(chan string)

	go mainLoop(sig, ticker, line)

	Run(storage)

}
