package checkbot

import (
	//	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/felicson/checkbot/internal/firewall"
)

var bot = "sdf.spider-141-8-132-86.google.com."
var botinvalid = "sdf.spider-141-8-132-86.bla.ru."
var logline = []byte(`207.46.13.16 - - [07/Mar/2016:17:26:23 +0300] "GET /board/gidrocilindr-55102-8603010-no-105684.html HTTP/1.1" 200 9518 "-" "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)"`)

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

func TestExtractIP(t *testing.T) {
	expect := LogRecord{
		IP:         net.ParseIP("207.46.13.16"),
		Path:       "/board/gidrocilindr-55102-8603010-no-105684.html",
		Date:       time.Date(2016, 03, 07, 0, 0, 0, 0, time.UTC),
		Bytes:      9518,
		StatusCode: 200,
	}
	got, err := ExtractIP([]byte(logline))
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(got, expect) {
		t.Fatalf("expect: %v, got: %v", expect, got)
	}
}

func TestToday(t *testing.T) {
	expect, _ := time.Parse("02/Jan/2006", time.Now().Format("02/Jan/2006"))
	today := today()
	if !expect.Equal(today) {
		t.Fatalf("expect: %s, got: %s", expect, today)
	}
	t.Log(expect, today)
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
		_, _ = ExtractIP(logline)
	}
}

func BenchmarkSplitN(b *testing.B) {

	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		_ = splitN(logline)
	}
}
