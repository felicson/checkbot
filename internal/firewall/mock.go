package firewall

import "fmt"

type Mock struct {
}

func (f *Mock) AddIP(ip string) error {
	//fmt.Printf("sudo /sbin/ipset add blacklist %s\n", ip)
	return nil
}

func (f *Mock) RemoveIP(ip string) error {
	fmt.Printf("sudo /sbin/ipset del blacklist %s\n", ip)
	return nil
}
