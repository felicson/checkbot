package checkbot

import (
	"log"
	"os"
)

//LogFile present web server logfile
type LogFile struct {
	offset int64
	File   *os.File
	Path   string
}

func (l *LogFile) SetOffset() {

	l.offset, _ = fileSize(l.File)
}

func (l *LogFile) Seek() bool {

	fsize, _ := fileSize(l.File)

	if fsize == l.offset {
		return false
	}

	if fsize < l.offset {
		l.offset = 0
	}

	if _, err := l.File.Seek(l.offset, 0); err != nil {
		log.Println(err)
		return false
	}
	return true
}

func fileSize(file *os.File) (int64, error) {

	fstat, err := file.Stat()

	if err != nil {
		return 0, err
	}

	return fstat.Size(), err

}
