package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/netip"

	"essaim.dev/essaim/client"
	"essaim.dev/essaim/clock"
	"essaim.dev/essaim/kinect"
	"golang.org/x/exp/shiny/driver"
)

var (
	addrFlag string
)

func init() {
	flag.StringVar(&addrFlag, "addr", "224.76.78.75:20809", "ip address and port used to send instructions")
}

func main() {
	flag.Parse()

	k, err := kinect.NewClient()
	if err != nil {
		log.Fatalf("could not not create kinect client: %s", err)
	}

	addr, err := netip.ParseAddrPort(addrFlag)
	if err != nil {
		log.Fatalf("could not not parse ip address: %s", err)
	}

	linkClock := clock.NewLinkClock(120.0)
	defer linkClock.Close()

	c, err := client.New(linkClock, 16, addr, k.RenderImage)
	if err != nil {
		log.Fatalf("could not create mikro controller: %s", err)
	}
	defer c.Close()

	linkClock.Start()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kinectStopped := make(chan error, 1)
	go func() {
		if err := k.Run(ctx); err != nil {
			kinectStopped <- fmt.Errorf("could not run kinect rendered: %s", err)
		}
	}()

	clientStopped := make(chan error, 1)
	go func() {
		if err := c.Run(ctx); err != nil {
			clientStopped <- fmt.Errorf("could not run client: %s", err)
		}
	}()

	driver.Main(c.Display)
}

// func render(img *image.RGBA, c color.Color) {
// 	draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
// }
