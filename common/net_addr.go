package common

import (
	"github.com/DSiSc/craft/log"
	"net"
	"strconv"
	"strings"
)

// NetAddress network address
type NetAddress struct {
	Protocol string
	IP       string
	Port     int32
}

// NewNetAddress create a new net address instance
func NewNetAddress(proto, ip string, port int32) *NetAddress {
	return &NetAddress{
		Protocol: proto,
		IP:       ip,
		Port:     port,
	}
}

// Equal check wheter two is equal
func (addr *NetAddress) Equal(another *NetAddress) bool {
	return (addr.IP == another.IP) && (addr.Port == another.Port)
}

// ParseNetAddress parse net address from address string
func ParseNetAddress(addrStr string) (*NetAddress, error) {
	var proto, address string
	if strings.Contains(addrStr, "://") {
		proto = strings.Split(addrStr, "://")[0]
		address = strings.Split(addrStr, "://")[1]
	} else {
		proto = "tcp" //default Protocol
		address = addrStr
	}

	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		log.Warn("invalid persistent peer address")
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Warn("invalid persistent peer Port")
		return nil, err
	}

	return NewNetAddress(proto, host, int32(port)), nil
}

//ToString encode netaddress to string
func (net *NetAddress) ToString() string {
	return net.Protocol + "://" + net.IP + ":" + strconv.Itoa(int(net.Port))
}
