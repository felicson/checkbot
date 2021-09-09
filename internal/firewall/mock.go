package firewall

import "fmt"

type Mock struct {
}

func (f *Mock) AddIP(ip string) {
	//fmt.Printf("sudo /sbin/ipset add blacklist %s\n", ip)
}

func (f *Mock) RemoveIP(ip string) {
	fmt.Printf("sudo /sbin/ipset del blacklist %s\n", ip)
}
