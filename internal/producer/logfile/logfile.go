package logfile

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/felicson/checkbot"
	"github.com/felicson/checkbot/internal/producer"
)

type eventFn func(event []byte) error
type LogFile struct {
	logs    []*checkbot.LogFile
	eventFn eventFn
}

var bufPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 512))
	},
}

func NewProducer(logs []string, eventParser eventFn, interval time.Duration) (LogFile, error) {
	l := LogFile{eventFn: eventParser}

	for _, srcLog := range logs {
		l.logs = append(l.logs, &checkbot.LogFile{Path: srcLog})
	}

	ticker := time.NewTicker(interval)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Kill, syscall.SIGTERM, os.Interrupt)

	go func() {
		for {
			select {
			case <-ticker.C:
				l.logsReader()
			case <-sig:
				ticker.Stop()
			}
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

			tmpFile, err := copyToPool(webLog)
			if err != nil {
				log.Printf("err on make tmp file: %v", err)
				return
			}
			scann := bufio.NewScanner(tmpFile)

			for scann.Scan() {
				if err := l.eventFn(scann.Bytes()); err != nil {
					log.Printf("on call eventFn err: %v\n", err)
				}
			}
			putBuffer(tmpFile)
			if scann.Err() != nil {
				log.Printf("pool buffer scan err: %v\n", err)
			}
		}(lf)
	}
}

func copyToPool(src io.Reader) (*bytes.Buffer, error) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	if _, err := io.Copy(buf, src); err != nil {
		return nil, err
	}
	return buf, nil
}

func putBuffer(buf *bytes.Buffer) {
	const maxSize = 1 << 16 //64kiB
	if buf.Cap() > maxSize {
		return
	}
	bufPool.Put(buf)
}
