package logfile

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/felicson/checkbot"
	"github.com/felicson/checkbot/internal/producer"
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

func (l *LogFile) SearchByPattern(pattern string) (producer.Matchers, error) {

	var matches = make(producer.Matchers)

	for _, log := range l.logs {

		file, err := os.Open(log.Path)
		if err != nil {
			return nil, err
		}
		scan := bufio.NewScanner(file)

		var tmp []string
		for scan.Scan() {
			line := scan.Text()
			if strings.Contains(line, pattern) {
				tmp = append(tmp, line)
			}
		}
		if err := file.Close(); err != nil {
			return nil, err
		}
		if err := scan.Err(); err != nil {
			return nil, err
		}
		matches[log.Path] = tmp
	}
	return matches, nil
}

func (l *LogFile) logsReader() {

	for _, lf := range l.logs {

		func(lf *checkbot.LogFile) {

			webLog, err := os.Open(lf.Path)

			if err != nil {
				log.Printf("err on open file %q - %v", lf.Path, err)
				return
			}
			defer webLog.Close()

			if !lf.Seek(webLog) {
				return
			}

			tmpFile, err := copyToTemporaryFile(webLog)
			if err != nil {
				log.Printf("err on make tmp file: %v", err)
				return
			}
			defer func() {
				tmpFile.Close()
				os.Remove(tmpFile.Name())
			}()
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

func copyToTemporaryFile(src io.Reader) (*os.File, error) {

	tmpFile, err := os.CreateTemp("/tmp", "checkbot_")
	if err != nil {
		return nil, fmt.Errorf("err on make tmp file: %v", err)
	}
	written, err := io.Copy(tmpFile, src)
	if err != nil {
		return nil, fmt.Errorf("err on copy of tail of log file to tmp file: %v", err)
	}
	log.Printf("written %d bytes to the %s\n", written, tmpFile.Name())

	if _, err := tmpFile.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("on seek tmp file: %v", err)
	}
	return tmpFile, nil
}
