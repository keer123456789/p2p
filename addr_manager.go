package p2p

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DSiSc/craft/log"
	"github.com/DSiSc/p2p/common"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	// addresses under which the address book will claim To need more addresses.
	needAddressThreshold = 1000
	syncInterval         = 2 * time.Minute
	// getAddrMax is the most addresses that we will send in response
	// To a getAddr (in practise the most addresses we will return From a
	// call To AddressCache()).
	getAddrMax = 2500
)

// AddressManager is used To manage neighbor's address
type AddressManager struct {
	filePath  string
	ourAddrs  map[string]*common.NetAddress
	addresses map[string]*common.NetAddress
	lock      sync.RWMutex
	changed   bool
	quitChan  chan interface{}
}

// NewAddressManager create an address manager instance
func NewAddressManager(filePath string) *AddressManager {
	addresses := loadAddress(filePath)
	return &AddressManager{
		filePath:  filePath,
		ourAddrs:  make(map[string]*common.NetAddress),
		addresses: addresses,
		quitChan:  make(chan interface{}),
	}
}

// AddOurAddress add our local address.
func (addrManager *AddressManager) AddOurAddress(addr *common.NetAddress) {
	addrManager.lock.Lock()
	defer addrManager.lock.Unlock()
	if addrManager.ourAddrs[addr.ToString()] != nil {
		return
	}
	addrManager.ourAddrs[addr.ToString()] = addr
}

// AddOurAddress add our local address.
func (addrManager *AddressManager) AddLocalAddress(port int32) error {
	addrManager.lock.Lock()
	defer addrManager.lock.Unlock()
	localIps, err := getLocalAddresses()
	if err != nil {
		return fmt.Errorf("failed To add our local address To address manager as:%v", err)
	}
	for _, localIp := range localIps {
		netAddr, err := common.ParseNetAddress(localIp + ":" + strconv.Itoa(int(port)))
		if err != nil {
			continue
		}
		if addrManager.ourAddrs[netAddr.ToString()] == nil {
			addrManager.ourAddrs[netAddr.ToString()] = netAddr
		}
	}
	return nil
}

// OurAddresses get local address.
func (addrManager *AddressManager) OurAddresses() []*common.NetAddress {
	addrManager.lock.RLock()
	defer addrManager.lock.RUnlock()
	addrs := make([]*common.NetAddress, 0)
	for _, addr := range addrManager.ourAddrs {
		addrs = append(addrs, addr)
	}
	return addrs
}

// IsOurAddress check whether the address is our address
func (addrManager *AddressManager) IsOurAddress(addr *common.NetAddress) bool {
	addrManager.lock.RLock()
	defer addrManager.lock.RUnlock()
	return addrManager.ourAddrs[addr.ToString()] != nil
}

// AddAddresses add new addresses
func (addrManager *AddressManager) AddAddresses(addrs []*common.NetAddress) {
	log.Debug("add %d addresses To book", len(addrs))
	addrManager.lock.Lock()
	defer addrManager.lock.Unlock()
	for _, addr := range addrs {
		if addrManager.ourAddrs[addr.ToString()] != nil {
			continue
		}

		if addrManager.addresses[addr.ToString()] != nil {
			continue
		}
		addrManager.addresses[addr.ToString()] = addr
		addrManager.changed = true
	}
}

// AddAddress add a new address
func (addrManager *AddressManager) AddAddress(addr *common.NetAddress) {
	log.Debug("add new address %s To book", addr.ToString())
	addrManager.lock.Lock()
	defer addrManager.lock.Unlock()
	if addrManager.ourAddrs[addr.ToString()] != nil {
		return
	}

	if addrManager.addresses[addr.ToString()] != nil {
		return
	}
	addrManager.addresses[addr.ToString()] = addr
	addrManager.changed = true
}

// RemoveAddress remove an address
func (addrManager *AddressManager) RemoveAddress(addr *common.NetAddress) {
	addrManager.lock.Lock()
	defer addrManager.lock.Unlock()
	delete(addrManager.addresses, addr.ToString())
	addrManager.changed = true
}

// GetAddress get a random address
func (addrManager *AddressManager) GetAddress() (*common.NetAddress, error) {
	addrs := addrManager.GetAllAddress()
	if len(addrs) > 0 {
		index := rand.Intn(len(addrs))
		return addrs[index], nil
	}
	return nil, errors.New("no address in address book")
}

// GetAddresses get a random address list To send To peer
func (addrManager *AddressManager) GetAddresses() []*common.NetAddress {
	addrs := addrManager.GetAllAddress()
	if addrManager.GetAddressCount() <= getAddrMax {
		return addrs
	} else {
		for i := 0; i < getAddrMax; i++ {
			j := rand.Intn(getAddrMax-i) + i
			addrs[i], addrs[j] = addrs[j], addrs[i]
		}
		return addrs[:getAddrMax]
	}
}

// GetAddressCount get address count
func (addrManager *AddressManager) GetAddressCount() int {
	addrManager.lock.RLock()
	defer addrManager.lock.RUnlock()
	return len(addrManager.addresses)
}

// GetAllAddress get all address
func (addrManager *AddressManager) GetAllAddress() []*common.NetAddress {
	addrManager.lock.RLock()
	defer addrManager.lock.RUnlock()
	addresses := make([]*common.NetAddress, 0)
	if len(addrManager.addresses) <= 0 {
		return addresses
	}
	for _, addr := range addrManager.addresses {
		addresses = append(addresses, addr)
	}
	return addresses
}

// NeedMoreAddrs check whether need more address.
func (addrManager *AddressManager) NeedMoreAddrs() bool {
	return addrManager.GetAddressCount() < needAddressThreshold
}

// Save save addresses To file
func (addrManager *AddressManager) Save() {
	addrManager.lock.Lock()
	defer addrManager.lock.Unlock()
	if !addrManager.changed {
		return
	}

	addrStrs := make([]string, 0)
	for addrStr, _ := range addrManager.addresses {
		addrStrs = append(addrStrs, addrStr)
	}

	buf, err := json.Marshal(addrStrs)
	fmt.Println(string(buf))
	if err != nil {
		log.Warn("failed To marshal recent addresses, as: %v", err)
	}

	err = ioutil.WriteFile(addrManager.filePath, buf, os.ModePerm)
	if err != nil {
		log.Warn("failed To write recent addresses To file, as: %v", err)
	}

	addrManager.changed = false
}

// Start start address manager
func (addrManager *AddressManager) Start() {
	go addrManager.saveHandler()
}

// Stop stop address manager
func (addrManager *AddressManager) Stop() {
	close(addrManager.quitChan)
}

// saveHandler save addresses To file periodically
func (addrManager *AddressManager) saveHandler() {
	saveFileTicker := time.NewTicker(syncInterval)
	for {
		select {
		case <-saveFileTicker.C:
			addrManager.Save()
		case <-addrManager.quitChan:
			return
		}
	}
}

// loadAddress load addresses From file.
func loadAddress(filePath string) map[string]*common.NetAddress {
	addrStrs := make([]string, 0)
	addresses := make(map[string]*common.NetAddress)
	if _, err := os.Stat(filePath); err != nil {
		return addresses
	}
	buf, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Error("failed To read address book file, as: %v", err)
		return addresses
	}

	err = json.Unmarshal(buf, &addrStrs)
	if err != nil {
		log.Error("failed To parse address book file, as %v", err)
		return addresses
	}

	for _, addrStr := range addrStrs {
		addr, err := common.ParseNetAddress(addrStr)
		if err != nil {
			log.Warn("encounter an invalid address %s", addrStr)
			continue
		}
		addresses[addrStr] = addr
	}
	log.Debug("load %d addresses From file %s", len(addresses), filePath)
	return addresses
}

// get all address of our server
func getLocalAddresses() ([]string, error) {
	ips := make([]string, 0)
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Error("failed To get system's interfaces")
		return nil, errors.New("failed To get system's interfaces")
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			log.Warn("failed To get interface's address")
			continue
		}
		// handle err
		for _, addr := range addrs {

			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip.To4() == nil || ip.IsLoopback() {
				log.Warn("skip invalid address %s", ip)
				continue
			}
			ips = append(ips, ip.String())
		}
	}
	return ips, nil
}
