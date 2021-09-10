package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"time"

	"github.com/felicson/checkbot"
	"github.com/felicson/checkbot/internal/firewall"
	"github.com/felicson/checkbot/internal/flags"
	logproducer "github.com/felicson/checkbot/internal/producer/logfile"
	"github.com/felicson/checkbot/internal/web"
)

var (
	logfile string
	loglist string
	wlist   flags.Whitelist
)

func main() {

	flag.StringVar(&loglist, "loglist", "/home/felicson/loglist.conf", "loglist=/path/loglist.conf")
	flag.StringVar(&logfile, "logfile", "/home/felicson/checkbot.log", "logfile=/path/loglist.conf")
	flag.Var(&wlist, "ignoreip", "ignoreip=1.2.3.4")

	flag.Parse()
	if flag.NFlag() < 2 {
		flag.Usage()
		return
	}

	done := make(chan bool, 1)

	banlogout, err := os.OpenFile(logfile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0664)

	if err != nil {
		log.Fatal("Cant open logfile")
	}
	defer banlogout.Close()

	log.SetOutput(banlogout)
	log.SetPrefix("checkbot: ")

	if err != nil {
		log.Fatal(err)
	}

	file, err := os.Open(loglist)
	if err != nil {
		log.Fatal(err)
	}

	reader := bufio.NewScanner(file)

	var logs []string
	for reader.Scan() {
		logs = append(logs, reader.Text())
	}
	file.Close()
	firewaller := &firewall.Mock{}

	users, err := checkbot.NewUsers(firewaller, wlist)
	if err != nil {
		log.Fatalf("on new users %v\n", err)
	}

	producer, err := logproducer.NewProducer(logs, users.HandleEvent, 2*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	server := web.NewServer(users, &producer, firewaller)

	if err := server.Run(); err != nil {
		log.Fatal(err)
	}
	defer server.Stop()
	<-done
}
