package checkbot

import (
	//	"fmt"
	"testing"

	"github.com/felicson/checkbot/internal/firewall"
)

var bot = "sdf.spider-141-8-132-86.google.com."
var botinvalid = "sdf.spider-141-8-132-86.bla.ru."
var logline = `207.46.13.16 - - [07/Mar/2016:17:26:23 +0300] "GET /board/gidrocilindr-55102-8603010-no-105684.html HTTP/1.1" 200 9518 "-" "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)"`

func TestBotValid(t *testing.T) {

	t.Log("Except yandex.ru")

	if !isBotValid(bot) {
		t.Errorf("Bot invalid ")
	}
	if isBotValid(botinvalid) {
		t.Errorf("Bot 2  invalid ")
	}
}

func TestIsWhitePath(t *testing.T) {
	tests := []struct {
		name   string
		expect bool
	}{
		{name: "/st-324.css", expect: true},
		{name: "/unknown/", expect: false},
	}
	users, _ := NewUsers(&firewall.Mock{}, nil)
	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			if got := users.IsWhitePath(tc.name); got != tc.expect {
				t.Fatalf("expect %v, got: %v", tc.expect, got)
			}
		})

	}
}

func BenchmarkBotValid(b *testing.B) {

	for n := 0; n < b.N; n++ {
		isBotValid(bot)
	}
}

func BenchmarkIsWhitePath(b *testing.B) {

	data, _ := NewUsers(&firewall.Mock{}, nil)
	path := "/ajax/test"
	for n := 0; n < b.N; n++ {
		data.IsWhitePath(path)
	}
}

//func BenchmarkItemLookup(b *testing.B) {
//
//	data := NewUser("1.2.3.4", 100)
//
//	for n := 0; n < b.N; n++ {
//		data.Lookup("1.2.3.4")
//	}
//}

func BenchmarkExtractIP(b *testing.B) {

	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		_, _ = ExtractIP([]byte(logline))
	}
}
