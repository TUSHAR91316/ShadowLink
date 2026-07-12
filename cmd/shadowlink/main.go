package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
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
	isEntry := flag.Bool("entry", false, "Run as an entry node (client)")
	isRelay := flag.Bool("relay", false, "Run as a relay node")
	isExit := flag.Bool("exit", false, "Run as an exit node")
	setProxy := flag.Bool("sysproxy", false, "Set system proxy on Windows")
	resetProxy := flag.Bool("reset-proxy", false, "Reset system proxy and exit")
	flag.Parse()

	if *resetProxy {
		sysproxy.Disable()
		return
	}

	if !*isEntry && !*isRelay && !*isExit {
		log.Println("No roles specified. Defaulting to --entry")
		*isEntry = true
	}

	checkEULA()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("Starting ShadowLink node (Entry: %v, Relay: %v, Exit: %v)", *isEntry, *isRelay, *isExit)

	// Initialize Discovery Service (DHT)
	// Bootstrap nodes are well-known libp2p peers used to seed the DHT.
	// Without these, a fresh node cannot discover any peers.
	bootstrapNodes := []string{
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
	}
	ds, err := discovery.NewDiscoveryService(ctx, *port, bootstrapNodes)
	if err != nil {
		log.Fatalf("Failed to initialize discovery service: %v", err)
	}

	// Announce role to the DHT and setup stream handlers
	if *isRelay || *isExit {
		if *isRelay {
			if err := ds.Announce(ctx, "shadowlink-relay"); err != nil {
				log.Printf("Failed to announce relay role: %v", err)
			}
		}
		if *isExit {
			if err := ds.Announce(ctx, "shadowlink-exit"); err != nil {
				log.Printf("Failed to announce exit role: %v", err)
			}
		}

		// I-3 / Bug 4 Fix: Pass the discovery service into the handler so relay
		// nodes can query the DHT to find exit nodes for 3-hop circuit building.
		ds.Host.SetStreamHandler(network.ProtocolID, func(s libp2pnet.Stream) {
			role := "relay"
			if *isExit {
				role = "exit"
			}
			// Each stream is handled in its own goroutine — already managed by libp2p
			network.HandleStream(s, role, ds)
		})
	}


	// If entry node, set up local SOCKS5 proxy
	if *isEntry {
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
				// Bug 5 Fix: Ensure proxy is disabled on exit and log any cleanup error.
				defer func() {
					log.Println("Disabling system proxy...")
					if err := sysproxy.Disable(); err != nil {
						log.Printf("Warning: Failed to disable system proxy: %v", err)
					}
				}()
			}
		}
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down ShadowLink gracefully...")
	cancel() // Cancel all active dials and contexts
	ds.Host.Close() // Immediately free the libp2p port binding
}

func checkEULA() {
	eulaFile := ".shadowlink_accepted"
	if _, err := os.Stat(eulaFile); err == nil {
		return // Already accepted
	}

	fmt.Println("================================================================================")
	fmt.Println("         SHADOWLINK END USER LICENSE AGREEMENT & TERMS OF SERVICE")
	fmt.Println("================================================================================")
	fmt.Println("WARNING: By using this software, you agree to the following terms:")
	fmt.Println("")
	fmt.Println("1. NO INFRASTRUCTURE: ShadowLink is an open-source protocol. The developers ")
	fmt.Println("   operate NO network servers and have NO control over peer-to-peer traffic.")
	fmt.Println("2. ABSOLUTE LIMITATION OF LIABILITY: The developers assume ABSOLUTELY ZERO ")
	fmt.Println("   LIABILITY for any damages, legal repercussions, or network traffic.")
	fmt.Println("3. COMPLIANCE: You assume 100% of the legal risk. You agree NOT to use this ")
	fmt.Println("   software to violate any local, national, or international laws.")
	fmt.Println("4. EXIT NODES: Running an Exit Node exposes your IP to third-party traffic.")
	fmt.Println("   You do so ENTIRELY at your own personal and legal risk.")
	fmt.Println("")
	fmt.Println("You must read the full TERMS_AND_CONDITIONS.md file in the root directory.")
	fmt.Println("================================================================================")
	fmt.Print("To legally bind yourself to these terms, type 'I ACCEPT' and press Enter: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input != "I ACCEPT" {
		fmt.Println("You did not accept the Terms & Conditions. Exiting.")
		os.Exit(1)
	}

	os.WriteFile(eulaFile, []byte("accepted"), 0644)
	fmt.Println("Terms accepted. Starting ShadowLink...")
}
