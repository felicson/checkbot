package main

import (
	"net/http"

	"github.com/shirou/gopsutil/process"
)

type ProcessInfo struct {
	PID     int32
	Name    string
	Cmdline string
	User    string
	CPU     float32
	Mem     float32
}

func processHandler(w http.ResponseWriter, req *http.Request) {

	processes, err := process.Processes()
	if err != nil {
		panic(err)
	}
	var processInfos []*ProcessInfo

	for i := range processes {
		process := processes[i]
		name, _ := process.Name()
		user, _ := process.Username()
		cpu, _ := process.CPUPercent()
		mem, _ := process.MemoryPercent()
		cmdline, _ := process.Cmdline()

		processInfos = append(processInfos, &ProcessInfo{process.Pid, name, cmdline, user, float32(cpu), mem})
	}
	renderTemplate(w, "processes", processInfos)
}
