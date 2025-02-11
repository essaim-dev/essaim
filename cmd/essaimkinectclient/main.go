package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/netip"

	"essaim.dev/essaim/client"
	"essaim.dev/essaim/clock"
	"essaim.dev/essaim/depthstream"
	"golang.org/x/exp/shiny/driver"
)

var (
	addrFlag       string
	streamAddrFlag string
	channelFlag    uint64
)

func init() {
	flag.StringVar(&addrFlag, "addr", "224.76.78.75:20809", "ip address and port used to send instructions")
	flag.StringVar(&streamAddrFlag, "stream-addr", "224.76.78.75:20810", "ip address and port used to send instructions")
	flag.Uint64Var(&channelFlag, "channel", 0, "")

}

func main() {
	flag.Parse()

	k, err := depthstream.NewClient(netip.MustParseAddrPort(streamAddrFlag))
	if err != nil {
		log.Fatalf("could not not create kinect client: %s", err)
	}

	linkClock := clock.NewLinkClock(120.0)
	defer linkClock.Close()

	c, err := client.New(linkClock, 16, netip.MustParseAddrPort(addrFlag), k.RenderImage, channelFlag)
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
			kinectStopped <- fmt.Errorf("could not run depthstream client: %s", err)
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
