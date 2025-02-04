package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/netip"

	"essaim.dev/essaim/clock"
	"essaim.dev/essaim/dmxclient"
)

var (
	interfaceFlag string
	addrFlag      string
)

func init() {
	flag.StringVar(&interfaceFlag, "interface", "eth0", "")
	flag.StringVar(&addrFlag, "addr", "224.2.2.3:9999", "ip address and port used to send instructions")
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("error: %s\n", err)
	}
}

func run() error {
	flag.Parse()

	addr, err := netip.ParseAddrPort(addrFlag)
	if err != nil {
		log.Fatalf("could not not parse ip address: %s", err)
	}

	iface, err := net.InterfaceByName(interfaceFlag)
	if err != nil {
		log.Fatalf("could not not find interface with given name: %s", err)
	}

	linkClock := clock.NewLinkClock(120.0)
	defer linkClock.Close()

	c, err := dmxclient.New(linkClock, 16, iface, addr)
	if err != nil {
		return fmt.Errorf("could not start dmx client: %w", err)
	}

	linkClock.Start()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientStopped := make(chan error, 1)
	go func() {
		if err := c.Run(ctx); err != nil {
			clientStopped <- fmt.Errorf("error while running client: %s", err)
		}
	}()

	if err := <-clientStopped; err != nil {
		return fmt.Errorf("client stopped: %w", err)
	}

	return nil
}
