package p2p

import (
	"github.com/DSiSc/p2p/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"os"
	"strconv"
	"testing"
)

func mockNetAddresses(num int) []*common.NetAddress {
	addrs := make([]*common.NetAddress, 0)
	for i := 1; i < 255; i++ {
		for j := 1; j < 255; j++ {
			for k := 0; k < 255; k++ {
				for l := 0; l < 255; l++ {
					addrs = append(addrs,
						&common.NetAddress{
							Protocol: "tcp",
							IP:       strconv.Itoa(i) + "," + strconv.Itoa(j) + "," + strconv.Itoa(k) + "," + strconv.Itoa(l),
							Port:     8080,
						})
					num--
					if num <= 0 {
						return addrs
					}
				}
			}
		}
	}
	return addrs
}

const addressFile = "address.json"

func TestMain(m *testing.M) {
	m.Run()
	os.Remove(addressFile)
}

func TestNewAddressManager(t *testing.T) {
	assert := assert.New(t)
	addrManger := NewAddressManager(addressFile)
	assert.NotNil(addrManger)
	assert.Equal(addressFile, addrManger.filePath)
	os.RemoveAll(addressFile)
}

func TestAddressManager_AddOurAddress(t *testing.T) {
	assert := assert.New(t)
	addrManger := NewAddressManager(addressFile)
	assert.NotNil(addrManger)
	address := &common.NetAddress{
		Protocol: "tcp",
		IP:       "127.0.0.1",
		Port:     8080,
	}
	addrManger.AddOurAddress(address)
	assert.Equal(address, addrManger.OurAddress())
}

func TestAddressManager_AddAddress(t *testing.T) {
	assert := assert.New(t)
	addrManger := NewAddressManager(addressFile)
	assert.NotNil(addrManger)

	address := &common.NetAddress{
		Protocol: "tcp",
		IP:       "192.0.0.1",
		Port:     8080,
	}
	addrManger.AddAddress(address)

	var exist bool
	for _, addr1 := range addrManger.GetAllAddress() {
		if address.Equal(addr1) {
			exist = true
			break
		}
	}
	assert.True(exist)
}

func TestAddressManager_RemoveAddress(t *testing.T) {
	assert := assert.New(t)
	addrManger := NewAddressManager(addressFile)
	assert.NotNil(addrManger)

	address := &common.NetAddress{
		Protocol: "tcp",
		IP:       "127.0.0.1",
		Port:     8080,
	}
	addrManger.AddAddress(address)
	addrManger.RemoveAddress(address)
	var exist bool
	for _, addr1 := range addrManger.GetAllAddress() {
		if address.Equal(addr1) {
			exist = true
			break
		}
	}
	assert.False(exist)
}

func TestAddressManager_NeedMoreAddrs(t *testing.T) {
	assert := assert.New(t)
	addrManger := NewAddressManager(addressFile)
	assert.NotNil(addrManger)
	assert.True(addrManger.NeedMoreAddrs())

	addrs := mockNetAddresses(needAddressThreshold + 1)
	for _, addr := range addrs {
		addrManger.AddAddress(addr)
	}
	assert.False(addrManger.NeedMoreAddrs())
}

func TestAddressManager_Stop(t *testing.T) {
	assert := assert.New(t)
	addrManger := NewAddressManager(addressFile)
	assert.NotNil(addrManger)
	addrManger.Start()
	addrManger.Stop()
	select {
	case <-addrManger.quitChan:
	default:
		assert.Error(errors.New("Failed to stop address manager"))
	}
}

func TestAddressManager_Save(t *testing.T) {
	assert := assert.New(t)
	addrManger := NewAddressManager(addressFile)
	assert.NotNil(addrManger)
	address := &common.NetAddress{
		Protocol: "tcp",
		IP:       "127.0.0.1",
		Port:     8080,
	}
	addrManger.AddAddress(address)
	addrManger.Save()

	exist := false
	addrs := loadAddress(addressFile)
	for _, addr := range addrs {
		if addr.Equal(address) {
			exist = true
			break
		}
	}
	assert.True(exist)
}