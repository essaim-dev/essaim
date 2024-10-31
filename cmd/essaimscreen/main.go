package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"net/netip"

	"essaim.dev/essaim/client"
	"essaim.dev/essaim/clock"
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

	addr, err := netip.ParseAddrPort(addrFlag)
	if err != nil {
		log.Fatalf("could not not parse ip address: %s", err)
	}

	linkClock := clock.NewLinkClock(120.0)
	defer linkClock.Close()

	c, err := client.New(linkClock, 16, addr, render)
	if err != nil {
		log.Fatalf("could not create mikro controller: %s", err)
	}
	defer c.Close()

	linkClock.Start()

	clientStopped := make(chan error, 1)
	go func() {
		if err := c.Run(context.Background()); err != nil {
			clientStopped <- fmt.Errorf("could not start mikro controller: %s", err)
		}
	}()

	driver.Main(c.Display)

	if err := <-clientStopped; err != nil {
		log.Fatalf("client stopped with error: %s", err)
	}
}

func render(c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)

	return img
}
