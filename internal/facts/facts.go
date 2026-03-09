// Package facts collects system information on the sprout side.
package facts

import (
	"net"
	"os"
	"runtime"
)

// SystemFacts holds auto-collected system properties.
type SystemFacts struct {
	OS           string   `json:"os"`
	Arch         string   `json:"arch"`
	Hostname     string   `json:"hostname"`
	GoVersion    string   `json:"go_version"`
	NumCPU       int      `json:"num_cpu"`
	IPAddresses  []string `json:"ip_addresses"`
	KernelArch   string   `json:"kernel_arch"`
	SproutID     string   `json:"sprout_id,omitempty"`
}

// Collect gathers system facts from the local machine.
func Collect() SystemFacts {
	hostname, _ := os.Hostname()
	return SystemFacts{
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		Hostname:    hostname,
		GoVersion:   runtime.Version(),
		NumCPU:      runtime.NumCPU(),
		IPAddresses: localIPs(),
		KernelArch:  runtime.GOARCH,
	}
}

// localIPs returns all non-loopback unicast IP addresses.
func localIPs() []string {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		if ipNet.IP.To4() != nil || ipNet.IP.To16() != nil {
			ips = append(ips, ipNet.IP.String())
		}
	}
	return ips
}
