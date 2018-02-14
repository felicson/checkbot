package main

import (
	"bufio"
	"fmt"
	"github.com/martinolsen/go-whois"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	//	"os/exec"
	"testing"
)

func TestWhois(t *testing.T) {

	record, err := whois.Lookup("66.249.69.103")

	if err != nil {
		t.Error("Err")
	}
	fmt.Printf("%s", record.Get("CIDR"))

	//	t.Log(whois)

}

func TestLookup(t *testing.T) {

	fmt.Println(net.LookupAddr("5.143.231.18"))
}
func BenchmarkFindHandler(b *testing.B) {

	file, _ := os.Open("loglist.conf")

	defer file.Close()
	reader := bufio.NewScanner(file)

	//Logs = LogsList{}

	for reader.Scan() {
		Logs = append(Logs, &LogFile{0, &os.File{}, reader.Text()})

	}
	b.ReportAllocs()
	//	b.StopTimer()
	r := request(b, "/info/ip?find=31.23.124.93")

	for n := 0; n < b.N; n++ {
		rw := httptest.NewRecorder()
		FindHandler(rw, r)
	}

}

func BenchmarkRenderTemplate(b *testing.B) {

	storage = NewItems()

	b.StopTimer()
	for i := 0; i < 10000; i++ {
		num := rand.Intn(254)
		num1 := rand.Intn(254)
		num2 := rand.Intn(254)
		num3 := rand.Intn(254)
		ip := fmt.Sprintf("%d.%d.%d.%d", num, num1, num2, num3)
		storage.Push(ip, NewItem(ip, 100))
	}
	array := make([]*Item, len(storage.row))
	i := 0
	for _, v := range storage.row {
		array[i] = v
		i++
	}
	b.StartTimer()
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		rw := httptest.NewRecorder()
		renderTemplate(rw, "index", ItemsList{Items: array})
	}
}

func BenchmarkInfoHandler(b *testing.B) {

	b.ReportAllocs()
	b.StopTimer()
	r := request(b, "/info/?sort=hits")
	storage = NewItems()

	log, err := os.Open("./access.log")

	if err != nil {
		panic(err)
	}
	scann := bufio.NewScanner(log)

	for scann.Scan() {
		line := scann.Text()
		analyzer(&line)
	}
	b.StartTimer()

	for n := 0; n < b.N; n++ {
		rw := httptest.NewRecorder()
		storage.InfoHandler(rw, r)
	}

}

func request(t testing.TB, url string) *http.Request {

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}
