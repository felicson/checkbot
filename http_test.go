package main

import (
	"fmt"
	"net"
	"os/exec"
	"testing"
)

var bot string = "spider-141-8-132-86.sputnik.ru."

func TestWhois(t *testing.T) {

	//whois, err := whois.Whois("40.77.167.23", "whois.ripe.net", "whois.arin.net", "whois.pir.org")
	whois, err := exec.Command("/usr/bin/whois", "141.8.132.86").Output()

	if err != nil {
		t.Error("Err")
	}
	fmt.Println(string(whois))

	t.Log(whois)

}

func TestLookup(t *testing.T) {

	fmt.Println(net.LookupAddr("5.143.231.18"))
}

func TestBotValid(t *testing.T) {

	if !isBotValid(bot) {
		t.Error("Bot invalid")
	}
}

func BenchmarkBotValid(b *testing.B) {

	for n := 0; n < b.N; n++ {
		isBotValid(bot)
	}
}
