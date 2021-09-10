package flags

import (
	"errors"
	"fmt"
	"strings"
)

type Whitelist map[string]bool

func (i *Whitelist) String() string {
	return fmt.Sprint(*i)
}

func (i *Whitelist) Set(value string) error {
	if len(*i) > 0 {
		return errors.New("ignoreip flag already set")
	}
	if !strings.Contains(value, ".") {
		return errors.New("ignoreip flag has wrong value")
	}
	*i = make(Whitelist)
	for _, v := range strings.Split(value, ",") {
		(*i)[v] = true
	}
	return nil
}
