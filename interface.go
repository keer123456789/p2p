package p2p

import "github.com/DSiSc/p2p/message"

type P2PAPI interface {
	// BroadCast broad cast message to all neighbor peers
	BroadCast(msg message.Message)

	// Gather gather newest data from p2p network
	Gather(peerFilter PeerFilter, reqMsg message.Message) error

	// MessageChan get p2p's message channel, (Messages sent to the server will eventually be placed in the message channel)
	MessageChan() <-chan message.Message
}
