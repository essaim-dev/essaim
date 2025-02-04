package main

import (
	"context"
	"flag"
	"fmt"
	"image"
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
	flag.StringVar(&addrFlag, "addr", "224.2.2.3:9999", "ip address and port used to send instructions")
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

func flipVertical(img *image.RGBA) *image.RGBA {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Create a new image to store the flipped version
	flipped := image.NewRGBA(bounds)

	// Copy pixels from img to flipped in vertical flipped order
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Copy pixel from (x, y) to (x, height - y - 1)
			flipped.Set(x, height-y-1, img.At(x, y))
		}
	}

	return flipped
}
