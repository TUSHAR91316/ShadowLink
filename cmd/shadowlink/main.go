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

	libp2pnet "github.com/libp2p/go-libp2p/core/network"
	"github.com/shadowlink/core/internal/config"
	"github.com/shadowlink/core/internal/discovery"
	"github.com/shadowlink/core/internal/network"
	"github.com/shadowlink/core/internal/socks5"
	"github.com/shadowlink/core/internal/sysproxy"
)

func main() {
	// CLI flags — defaults sourced from config so they are never hardcoded here.
	port := flag.Int("port", config.DefaultP2PPort, "Port for incoming P2P connections (0 = OS-assigned)")
	socksPort := flag.Int("socks", config.DefaultSOCKSPort, "Port for the local SOCKS5 proxy")
	isEntry := flag.Bool("entry", false, "Run as an entry node (client-side proxy)")
	isRelay := flag.Bool("relay", false, "Run as a relay node (middleman hop)")
	isExit := flag.Bool("exit", false, "Run as an exit node (internet egress)")
	setProxy := flag.Bool("sysproxy", false, "Automatically configure the OS system proxy (Windows only)")
	resetProxy := flag.Bool("reset-proxy", false, "Reset the OS system proxy and exit immediately")
	flag.Parse()

	// Handle --reset-proxy before anything else so it exits cleanly.
	if *resetProxy {
		if err := sysproxy.Disable(); err != nil {
			log.Printf("Warning: failed to reset system proxy: %v", err)
		}
		return
	}

	// Default to entry node if no role is specified.
	if !*isEntry && !*isRelay && !*isExit {
		log.Println("No role specified — defaulting to --entry")
		*isEntry = true
	}

	checkEULA()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("Starting ShadowLink (entry=%v relay=%v exit=%v port=%d socks=%d)",
		*isEntry, *isRelay, *isExit, *port, *socksPort)

	// Initialise the libp2p host and Kad-DHT discovery service.
	ds, err := discovery.NewDiscoveryService(ctx, *port, config.DefaultBootstrapPeers)
	if err != nil {
		log.Fatalf("Failed to initialise discovery service: %v", err)
	}
	defer ds.Host.Close()

	// ── Relay / Exit node setup ────────────────────────────────────────────
	if *isRelay || *isExit {
		if *isRelay {
			if err := ds.Announce(ctx, config.RendezvousRelay); err != nil {
				log.Printf("Warning: failed to announce relay role: %v", err)
			}
		}
		if *isExit {
			if err := ds.Announce(ctx, config.RendezvousExit); err != nil {
				log.Printf("Warning: failed to announce exit role: %v", err)
			}
		}

		// Determine the role string for logging inside HandleStream.
		role := "relay"
		if *isExit && !*isRelay {
			role = "exit"
		} else if *isRelay && *isExit {
			role = "relay+exit"
		}

		// ctx is captured correctly here — the closure is set once, not per-call.
		ds.Host.SetStreamHandler(config.ProtocolID, func(s libp2pnet.Stream) {
			network.HandleStream(ctx, s, role, ds)
		})
	}

	// ── Entry node setup ───────────────────────────────────────────────────
	if *isEntry {
		log.Printf("Starting SOCKS5 proxy on 127.0.0.1:%d", *socksPort)

		dialer := func(dialCtx context.Context, netwk, addr string) (net.Conn, error) {
			return network.DialCircuit(dialCtx, ds, netwk, addr)
		}

		proxy, err := socks5.NewServer(*socksPort, dialer)
		if err != nil {
			log.Fatalf("Failed to create SOCKS5 proxy: %v", err)
		}

		go func() {
			if err := proxy.ListenAndServe(ctx); err != nil {
				log.Printf("SOCKS5 proxy error: %v", err)
			}
		}()

		if *setProxy {
			if err := sysproxy.EnableSOCKS5("127.0.0.1", *socksPort); err != nil {
				log.Printf("Warning: failed to set system proxy: %v", err)
			} else {
				log.Println("System proxy configured")
				// Restore proxy state on shutdown.
				defer func() {
					if err := sysproxy.Disable(); err != nil {
						log.Printf("Warning: failed to reset system proxy: %v", err)
					}
				}()
			}
		}
	}

	// ── Block until SIGINT / SIGTERM ───────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("Received signal %s — shutting down gracefully...", sig)
	cancel() // Cancel all active dials and DHT operations.
}

// checkEULA prompts the user to accept the Terms & Conditions on first run.
// A sentinel file (config.EULAFileName) is written to disk on acceptance.
// If the file already exists the function returns immediately.
func checkEULA() {
	if _, err := os.Stat(config.EULAFileName); err == nil {
		return // Already accepted
	}

	fmt.Println("================================================================================")
	fmt.Println("         SHADOWLINK END USER LICENSE AGREEMENT & TERMS OF SERVICE")
	fmt.Println("================================================================================")
	fmt.Println("WARNING: By using this software, you agree to the following terms:")
	fmt.Println()
	fmt.Println("1. NO INFRASTRUCTURE: ShadowLink is an open-source protocol. The developers")
	fmt.Println("   operate NO network servers and have NO control over peer-to-peer traffic.")
	fmt.Println("2. ABSOLUTE LIMITATION OF LIABILITY: The developers assume ABSOLUTELY ZERO")
	fmt.Println("   LIABILITY for any damages, legal repercussions, or network traffic.")
	fmt.Println("3. COMPLIANCE: You assume 100% of the legal risk. You agree NOT to use this")
	fmt.Println("   software to violate any local, national, or international laws.")
	fmt.Println("4. EXIT NODES: Running an Exit Node exposes your IP to third-party traffic.")
	fmt.Println("   You do so ENTIRELY at your own personal and legal risk.")
	fmt.Println()
	fmt.Println("Read the full TERMS_AND_CONDITIONS.md before proceeding.")
	fmt.Println("================================================================================")
	fmt.Print("Type 'I ACCEPT' and press Enter to agree: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("\nFailed to read input. Exiting.")
		os.Exit(1)
	}
	input = strings.TrimSpace(input)

	if input != "I ACCEPT" {
		fmt.Println("You did not accept the Terms & Conditions. Exiting.")
		os.Exit(1)
	}

	if err := os.WriteFile(config.EULAFileName, []byte("accepted\n"), 0o600); err != nil {
		log.Printf("Warning: could not write EULA acceptance file: %v", err)
	}
	fmt.Println("Terms accepted. Starting ShadowLink...")
}
