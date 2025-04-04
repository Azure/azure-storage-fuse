package distributedcache

import (
	"errors"
	"fmt"
	"net"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

var getNetAddrs = net.InterfaceAddrs

// logAndReturnError logs the error and returns it.
func logAndReturnError(msg string) error {
	log.Err(msg)
	return errors.New(msg)
}

// TODO: Interface name and identify the ip.
func getVmIp() (string, error) {
	addresses, err := getNetAddrs()
	if err != nil {
		return "", err
	}

	var vmIP string
	for _, addr := range addresses {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		if ipNet.IP.To4() != nil {
			vmIP = ipNet.IP.String()
			// parts := strings.Split(vmIP, ".")
			// vmIP = fmt.Sprintf("%s.%s.%d.%d", parts[0], parts[1], rand.Intn(256), rand.Intn(256))
			break
		}
	}
	if vmIP == "" {
		return "", fmt.Errorf("unable to find a valid non-loopback IPv4 address")
	}

	return vmIP, nil
}
