package config

// P2PConfig configuration of the p2p network.
type P2PConfig struct {
	AddrBookFilePath string //address book file path
	ListenAddress    string // server listen address
	MaxConnOutBound  int    // max connection out bound
	MaxConnInBound   int    // max connection in bound
	PersistentPeers  string // persistent peers
}
