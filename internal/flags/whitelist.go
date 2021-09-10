package flags

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrAlreadySet = errors.New("flag already defined")
	ErrWrongValue = errors.New("argument has wrong value")
)

type Whitelist map[string]bool

func (i *Whitelist) String() string {
	return fmt.Sprint(*i)
}

func (i *Whitelist) Set(value string) error {
	if len(*i) > 0 {
		return ErrAlreadySet
	}
	if !strings.Contains(value, ".") {
		return ErrWrongValue
	}
	values := strings.Split(value, ",")
	*i = make(Whitelist, len(values))
	for _, v := range values {
		(*i)[v] = true
	}
	return nil
}
