package main

import (
	//	"fmt"
	"testing"
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

func BenchmarkBotValid(b *testing.B) {

	for n := 0; n < b.N; n++ {
		isBotValid(bot)
	}
}

func BenchmarkIsWhitePath(b *testing.B) {

	data := NewItems()
	path := "/ajax/test"
	for n := 0; n < b.N; n++ {
		data.IsWhitePath(&path)
	}
}

func BenchmarkItemLookup(b *testing.B) {

	data := NewItem("1.2.3.4", 100)

	for n := 0; n < b.N; n++ {
		data.Lookup("1.2.3.4")
	}
}

func BenchmarkItemPush(b *testing.B) {

	b.StopTimer()
	data := NewItems()
	b.StartTimer()
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		data.Push("1.2.3.4", NewItem("12.34.23.3", 200))
	}
}

func BenchmarkExtractIP(b *testing.B) {

	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		_, _, _, _, _, _ = ExtractIP(&logline)
	}
}
