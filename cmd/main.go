package main

import (
	"bufio"
	"flag"
	"log"
	"os"

	"github.com/felicson/checkbot"
	"github.com/felicson/checkbot/internal/firewall"
	logproducer "github.com/felicson/checkbot/internal/producer/logfile"
	"github.com/felicson/checkbot/internal/web"
)

var (
	logfile string
	loglist string
	wlist   checkbot.Whitelist
)

func init() {

	flag.StringVar(&loglist, "loglist", "/home/felicson/loglist.conf", "loglist=/path/loglist.conf")
	flag.StringVar(&logfile, "logfile", "/home/felicson/checkbot.log", "logfile=/path/loglist.conf")
	flag.Var(&wlist, "ignoreip", "ignoreip=1.2.3.4")

}

func main() {

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

	users, err := checkbot.NewUsers(&firewall.Mock{}, wlist)
	if err != nil {
		log.Fatalf("on new items %v\n", err)
	}

	_, err = logproducer.NewProducer(logs, users.HandleEvent)
	if err != nil {
		log.Fatal(err)
	}

	server := web.NewServer(users)

	if err := server.Run(); err != nil {
		log.Fatal(err)
	}
	defer server.Stop()
	<-done
}
