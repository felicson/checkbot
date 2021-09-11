package firewall

import (
	"fmt"
	"os/exec"
)

type Ipset struct{}

func (f *Ipset) AddIP(ip string) error {
	return f.execCommand(fmt.Sprintf("sudo /sbin/ipset add blacklist %s", ip))
}

func (f *Ipset) RemoveIP(ip string) error {
	return f.execCommand(fmt.Sprintf("sudo /sbin/ipset del blacklist %s", ip))
}

func (f *Ipset) execCommand(arg string) error {
	return exec.Command("/bin/sh", "-c", arg).Run()
}
