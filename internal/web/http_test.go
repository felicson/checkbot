package web

import (
	"fmt"
	"net"
	"net/http"

	"github.com/martinolsen/go-whois"

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

func request(t testing.TB, url string) *http.Request {

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal(err)
	}
	return req
}
