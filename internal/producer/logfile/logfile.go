package logfile

import (
	"bufio"
	"io"
	"log"
	"os"
	"time"

	"github.com/felicson/checkbot"
)

type eventFn func(event string) error
type LogFile struct {
	ticker  *time.Ticker
	logs    []*checkbot.LogFile
	eventFn eventFn
}

func NewProducer(logs []string, efn eventFn) (LogFile, error) {
	ticker := time.NewTicker(2 * time.Second)
	l := LogFile{eventFn: efn}

	for _, srcLog := range logs {
		l.logs = append(l.logs, &checkbot.LogFile{File: &os.File{}, Path: srcLog})
	}

	go func() {
		for range ticker.C {
			l.logsReader()
		}
	}()
	return l, nil
}

func (l *LogFile) AnalyzeEvent(cb func(event string) error) error {
	l.eventFn = cb
	return nil
}

func (l *LogFile) logsReader() {

	for _, lf := range l.logs {

		func(lf *checkbot.LogFile, ef eventFn) {

			var err error

			lf.File, err = os.Open(lf.Path)

			if err != nil {
				log.Printf("err on open file %q - %v", lf.Path, err)
				return
			}
			defer lf.File.Close()

			if !lf.Seek() {
				return
			}

			lf.SetOffset()

			tmpFile, err := os.CreateTemp("/tmp", "checkbot_")
			if err != nil {
				log.Printf("err on make tmp file: %v", err)
				return
			}
			defer func() {
				tmpFile.Close()
				os.Remove(tmpFile.Name())
			}()
			if written, err := io.Copy(tmpFile, lf.File); err != nil {
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
				line := scann.Text()
				if err := ef(line); err != nil {
					log.Printf("on call eventFn err: %v\n", err)
				}
			}
			if scann.Err() != nil {
				log.Printf("scan err: %v\n", err)
			}
		}(lf, l.eventFn)
	}
}
