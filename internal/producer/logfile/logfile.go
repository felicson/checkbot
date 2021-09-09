package logfile

import (
	"bufio"
	"io"
	"log"
	"os"
	"time"

	"github.com/felicson/checkbot"
)

type eventFn func(event []byte) error
type LogFile struct {
	ticker  *time.Ticker
	logs    []*checkbot.LogFile
	eventFn eventFn
}

func NewProducer(logs []string, eventParser eventFn) (LogFile, error) {
	ticker := time.NewTicker(2 * time.Second)
	l := LogFile{eventFn: eventParser}

	for _, srcLog := range logs {
		l.logs = append(l.logs, &checkbot.LogFile{Path: srcLog})
	}

	go func() {
		for range ticker.C {
			l.logsReader()
		}
	}()
	return l, nil
}

func (l *LogFile) logsReader() {

	for _, lf := range l.logs {

		func(lf *checkbot.LogFile) {

			var err error

			webLog, err := os.Open(lf.Path)

			if err != nil {
				log.Printf("err on open file %q - %v", lf.Path, err)
				return
			}
			defer webLog.Close()

			if !lf.Seek(webLog) {
				return
			}

			tmpFile, err := os.CreateTemp("/tmp", "checkbot_")
			if err != nil {
				log.Printf("err on make tmp file: %v", err)
				return
			}
			defer func() {
				tmpFile.Close()
				os.Remove(tmpFile.Name())
			}()
			if written, err := io.Copy(tmpFile, webLog); err != nil {
				log.Printf("err on copy of tail of log file to tmp file: %v", err)
				return
			} else {
				log.Printf("written %d bytes, from %s to %s\n", written, lf.Path, tmpFile.Name())
			}
			if _, err := tmpFile.Seek(0, 0); err != nil {
				log.Println(err)
			}
			scann := bufio.NewScanner(tmpFile)

			for scann.Scan() {
				if err := l.eventFn(scann.Bytes()); err != nil {
					log.Printf("on call eventFn err: %v\n", err)
				}
			}
			if scann.Err() != nil {
				log.Printf("logfile %s, scan err: %v\n", tmpFile.Name(), err)
			}
		}(lf)
	}
}
