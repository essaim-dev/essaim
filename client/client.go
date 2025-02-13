package client

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"essaim.dev/essaim/clock"
	"essaim.dev/essaim/pattern"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/size"
)

const (
	refreshRate = time.Duration(time.Millisecond * 50)
)

type Client struct {
	clock clock.Clock
	conn  *net.UDPConn

	pattern   *pattern.ColorPattern
	patternMu sync.RWMutex

	currentStep atomic.Int32

	stopped      chan error
	refreshImage chan *image.RGBA

	renderFunc func(color.Color) *image.RGBA

	channel uint64
}

func New(
	clock clock.Clock,
	stepCount int,
	addr netip.AddrPort,
	renderFunc func(color.Color) *image.RGBA,
	channel uint64,
	iface *net.Interface,
) (*Client, error) {
	conn, err := net.ListenMulticastUDP("udp4", iface, net.UDPAddrFromAddrPort(addr))
	if err != nil {
		return nil, fmt.Errorf("could not listen on multicast address: %w", err)
	}
	conn.SetReadBuffer(512)

	return &Client{
		clock:        clock,
		conn:         conn,
		pattern:      pattern.NewColorPattern(stepCount),
		stopped:      make(chan error, 1),
		refreshImage: make(chan *image.RGBA),
		renderFunc:   renderFunc,
		channel:      channel,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Run(ctx context.Context) error {
	connStopped := make(chan error, 1)
	go c.consumeConn(connStopped)

	ticks := c.clock.Tick()

	refresh := time.NewTicker(refreshRate)

	for {
		select {
		case err := <-connStopped:
			return fmt.Errorf("error while listening for pattern updates: %w", err)

		case err := <-c.stopped:
			if err != nil {
				return fmt.Errorf("display stopped with error: %w", err)
			}
			return nil

		case tick := <-ticks:
			c.currentStep.Store(int32(tick % 16))

		case <-refresh.C:
			col, _ := c.pattern.ColorAt(int(c.currentStep.Load()))
			c.refreshImage <- c.renderFunc(col)

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *Client) consumeConn(stopped chan error) {
	b := make([]byte, 512)

	for {
		n, err := c.conn.Read(b)
		if err != nil {
			stopped <- fmt.Errorf("error while reading from udp: %w", err)
			return
		}

		c.patternMu.Lock()
		c.pattern.Decode(b[:n], c.channel)
		c.patternMu.Unlock()

	}
}

func (c *Client) Display(s screen.Screen) {
	w, err := s.NewWindow(&screen.NewWindowOptions{
		Title:  "Essaim",
		Width:  640,
		Height: 480,
	})
	if err != nil {
		c.stopped <- fmt.Errorf("could not create window: %w", err)
		return
	}
	defer w.Release()

	tex, err := s.NewTexture(image.Pt(640, 480))
	if err != nil {
		c.stopped <- fmt.Errorf("could not create texture: %w", err)
		return
	}
	defer tex.Release()

	buf, err := s.NewBuffer(image.Pt(640, 480))
	if err != nil {
		c.stopped <- fmt.Errorf("could not create buffer: %w", err)
		return
	}
	defer buf.Release()

	go publishRefreshEvent(w, c.refreshImage)

	sizeEvent := size.Event{}
	for {
		event := w.NextEvent()

		switch e := event.(type) {
		case lifecycle.Event:
			if e.To == lifecycle.StageDead {
				c.stopped <- nil
				return
			}

		case key.Event:
			if e.Code == key.CodeEscape {
				c.stopped <- nil
				return
			}

		case size.Event:
			sizeEvent = e

		case uploadEvent:
			copy(buf.RGBA().Pix, e.Pixels)
			tex.Upload(image.Point{}, buf, buf.Bounds())

		}

		w.Scale(sizeEvent.Bounds(), tex, tex.Bounds(), draw.Src, nil)
		w.Publish()

	}
}

func publishRefreshEvent(q screen.EventDeque, refreshImage chan *image.RGBA) {
	for i := range refreshImage {
		q.Send(uploadEvent{
			Pixels: i.Pix,
		})
	}
}

type uploadEvent struct {
	Pixels []uint8
}
