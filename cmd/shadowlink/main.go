package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/shadowlink/core/internal/discovery"
	"github.com/shadowlink/core/internal/network"
	"github.com/shadowlink/core/internal/socks5"
	"github.com/shadowlink/core/internal/sysproxy"
	libp2pnet "github.com/libp2p/go-libp2p/core/network"
)

func main() {
	port := flag.Int("port", 9000, "Port to listen on for P2P connections")
	socksPort := flag.Int("socks", 1080, "Port to listen on for SOCKS5 proxy")
	role := flag.String("role", "entry", "Role of this node: entry, relay, or exit")
	setProxy := flag.Bool("sysproxy", false, "Set system proxy on Windows")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("Starting ShadowLink node (Role: %s)", *role)

	// Initialize Discovery Service (DHT)
	bootstrapNodes := []string{} // E.g., "/ip4/1.2.3.4/tcp/9000/p2p/Qm..."
	ds, err := discovery.NewDiscoveryService(ctx, *port, bootstrapNodes)
	if err != nil {
		log.Fatalf("Failed to initialize discovery service: %v", err)
	}

	// Announce role to the DHT and setup stream handlers
	if *role == "exit" || *role == "relay" {
		err := ds.Announce(ctx, "shadowlink-"+*role)
		if err != nil {
			log.Fatalf("Failed to announce role: %v", err)
		}
		
		ds.Host.SetStreamHandler(network.ProtocolID, func(s libp2pnet.Stream) {
			network.HandleStream(s, *role)
		})
	}

	// If entry node, set up local SOCKS5 proxy
	if *role == "entry" {
		log.Printf("Starting local SOCKS5 proxy on port %d", *socksPort)

		dialer := func(ctx context.Context, netwk, addr string) (net.Conn, error) {
			return network.DialCircuit(ctx, ds, netwk, addr)
		}

		proxy, err := socks5.NewServer(*socksPort, dialer)
		if err != nil {
			log.Fatalf("Failed to initialize SOCKS5 proxy: %v", err)
		}
		
		go func() {
			if err := proxy.ListenAndServe(ctx); err != nil {
				log.Fatalf("SOCKS5 proxy stopped: %v", err)
			}
		}()
		
		if *setProxy {
			log.Println("Setting system proxy...")
			if err := sysproxy.EnableSOCKS5("127.0.0.1", *socksPort); err != nil {
				log.Printf("Warning: Failed to set system proxy: %v", err)
			} else {
				// Ensure proxy is disabled on exit
				defer func() {
					log.Println("Disabling system proxy...")
					sysproxy.Disable()
				}()
			}
		}
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down ShadowLink...")
}
