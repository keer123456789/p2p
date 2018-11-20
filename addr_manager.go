package p2p

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DSiSc/craft/log"
	"github.com/DSiSc/p2p/common"
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
	"time"
)

const (
	// addresses under which the address book will claim to need more addresses.
	needAddressThreshold = 1000
	syncInterval         = 2 * time.Minute
)

// AddressManager is used to manage neighbor's address
type AddressManager struct {
	filePath  string
	outAddr   *common.NetAddress
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
		addresses: addresses,
		quitChan:  make(chan interface{}),
	}
}

// AddOurAddress add our local address.
func (addrManager *AddressManager) AddOurAddress(addr *common.NetAddress) {
	addrManager.lock.Lock()
	defer addrManager.lock.Unlock()
	addrManager.outAddr = addr
}

// OurAddress get local address.
func (addrManager *AddressManager) OurAddress() *common.NetAddress {
	addrManager.lock.RLock()
	defer addrManager.lock.RUnlock()
	return addrManager.outAddr
}

// AddAddress add a new address
func (addrManager *AddressManager) AddAddress(addr *common.NetAddress) {
	addrManager.lock.Lock()
	defer addrManager.lock.Unlock()
	if addrManager.outAddr != nil && addrManager.outAddr.Equal(addr) {
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
	addrManager.lock.RLock()
	defer addrManager.lock.RUnlock()
	if len(addrManager.addresses) > 0 {
		index := rand.Intn(len(addrManager.addresses))
		for _, addr := range addrManager.addresses {
			if index <= 0 {
				return addr, nil
			}
			index--
		}
	}
	return nil, errors.New("no address in address book")
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

// Save save addresses to file
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
		log.Warn("failed to marshal recent addresses, as: %v", err)
	}

	err = ioutil.WriteFile(addrManager.filePath, buf, os.ModePerm)
	if err != nil {
		log.Warn("failed to write recent addresses to file, as: %v", err)
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

// saveHandler save addresses to file periodically
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

// loadAddress load addresses from file.
func loadAddress(filePath string) map[string]*common.NetAddress {
	addrStrs := make([]string, 0)
	addresses := make(map[string]*common.NetAddress)
	if _, err := os.Stat(filePath); err != nil {
		return addresses
	}
	buf, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Error("failed to read address book file, as: %v", err)
		return addresses
	}

	err = json.Unmarshal(buf, &addrStrs)
	if err != nil {
		log.Error("failed to parse address book file, as %v", err)
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

	return addresses
}
