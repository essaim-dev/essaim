package main

import (
	"context"
	"flag"
	"log"
	"net/netip"

	"essaim.dev/essaim/clock"
	"essaim.dev/essaim/depthstream"
)

var (
	addrFlag       string
	streamAddrFlag string
)

func init() {
	flag.StringVar(&addrFlag, "addr", "224.76.78.75:20809", "ip address and port used to send instructions")
	flag.StringVar(&streamAddrFlag, "stream-addr", "224.76.78.75:20810", "ip address and port used to send instructions")
}

func main() {
	flag.Parse()

	k, err := depthstream.NewServer(netip.MustParseAddrPort(streamAddrFlag))
	if err != nil {
		log.Fatalf("could not not create kinect client: %s", err)
	}

	linkClock := clock.NewLinkClock(120.0)
	defer linkClock.Close()

	linkClock.Start()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := k.Run(ctx); err != nil {
		log.Fatalf("could not run kinect rendered: %s", err)
	}
}

// func render(img *image.RGBA, c color.Color) {
// 	draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
// }
