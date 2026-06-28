package network

import (
	"bufio"
	"io"
	"log"
	"net"
	"strings"

	"github.com/libp2p/go-libp2p/core/network"
)

// HandleStream processes incoming libp2p streams.
// Exit nodes resolve the target and dial out to the internet.
func HandleStream(s network.Stream, role string) {
	log.Printf("New stream received from %s", s.Conn().RemotePeer())
	defer s.Close()

	if role != "exit" {
		log.Printf("Only exit node stream handling is implemented in this scaffold")
		return
	}

	// Read target address from stream
	reader := bufio.NewReader(s)
	targetAddr, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("Failed to read target addr: %v", err)
		return
	}
	targetAddr = strings.TrimSpace(targetAddr)
	log.Printf("Exit Node: Dialing out to %s", targetAddr)

	// Dial the target on the internet
	outConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("Exit Node: Failed to dial target: %v", err)
		return
	}
	defer outConn.Close()

	// Setup the libP2PConn wrapper to decrypt incoming / encrypt outgoing
	wrapper := &libP2PConn{
		Stream: s,
		Keys:   [][]byte{[]byte("12345678901234567890123456789012")},
	}

	// Proxy the data back and forth
	errc := make(chan error, 2)
	go func() {
		// Read decrypted data from peer, write to internet
		_, err := io.Copy(outConn, wrapper)
		errc <- err
	}()
	go func() {
		// Read from internet, encrypt, and write to peer
		_, err := io.Copy(wrapper, outConn)
		errc <- err
	}()

	<-errc
}
