package main

import (
	"context"
	"flag"
	"fmt"
	"net/netip"

	"essaim.dev/essaim/clock"
	"essaim.dev/essaim/mikrocontroller"
	// _ "net/http/pprof"
)

var (
	addrFlag string
)

func init() {
	flag.StringVar(&addrFlag, "addr", "224.19.29.39:4242", "ip address and port used to send instructions")
}

func main() {
	// f, err := os.Create("cpu.prof")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer f.Close()

	// if err := pprof.StartCPUProfile(f); err != nil {
	// 	log.Fatal(err)
	// }
	// defer pprof.StopCPUProfile()

	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

	if err := run(); err != nil {
		fmt.Printf("error: %s", err)
	}
}

func run() error {
	flag.Parse()

	addr, err := netip.ParseAddrPort(addrFlag)
	if err != nil {
		return fmt.Errorf("could not not parse ip address: %w", err)
	}

	linkClock := clock.NewLinkClock(120.0)
	defer linkClock.Close()

	c, err := mikrocontroller.NewController(linkClock, 16, addr)
	if err != nil {
		return fmt.Errorf("could not create mikro controller: %w", err)
	}
	defer c.Close()

	linkClock.Start()
	if err := c.Run(context.Background()); err != nil {
		return fmt.Errorf("could not run mikro controller: %w", err)
	}

	return nil
}
