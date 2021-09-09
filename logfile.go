package checkbot

import (
	"io"
	"log"
	"os"
)

//LogFile present web server logfile
type LogFile struct {
	offset int64
	File   *os.File
	Path   string
}

func (l *LogFile) Seek(rdr io.Seeker) bool {

	fsize, _ := fileSize(l.Path)

	if fsize == l.offset {
		return false
	}

	if fsize < l.offset {
		l.offset = 0
	}

	if _, err := rdr.Seek(l.offset, 0); err != nil {
		log.Printf("on seek error: %v\n", err)
		return false
	}
	l.offset = fsize

	return true
}

func fileSize(file string) (int64, error) {

	fstat, err := os.Stat(file)

	if err != nil {
		return 0, err
	}

	return fstat.Size(), err

}
