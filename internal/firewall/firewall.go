package firewall

import (
	"fmt"
	"os/exec"
)

type F struct {
}

func (f *F) AddIP(ip string) {
	f.execCommand(fmt.Sprintf("sudo /sbin/ipset add blacklist %s", ip))
}

func (f *F) RemoveIP(ip string) {
	f.execCommand(fmt.Sprintf("sudo /sbin/ipset del blacklist %s", ip))
}

func (f *F) execCommand(arg string) error {

	cmd := exec.Command("/bin/sh", "-c", arg)
	return cmd.Run()

}
