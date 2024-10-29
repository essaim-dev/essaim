package dmxclient

import (
	"context"
	"fmt"
	"image/color"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"essaim.dev/essaim/clock"
	"essaim.dev/essaim/dmx"
	"essaim.dev/essaim/pattern"
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

	dmxDevice *dmx.Device
}

func New(clock clock.Clock, stepCount int, addr netip.AddrPort) (*Client, error) {
	conn, err := net.ListenMulticastUDP("udp4", nil, net.UDPAddrFromAddrPort(addr))
	if err != nil {
		return nil, fmt.Errorf("could not listen on multicast address: %w", err)
	}
	conn.SetReadBuffer(512)

	dev, err := dmx.OpenDevice()
	if err != nil {
		return nil, fmt.Errorf("could not open dmx device: %w", err)
	}

	return &Client{
		clock:     clock,
		conn:      conn,
		pattern:   pattern.NewColorPattern(stepCount),
		dmxDevice: dev,
	}, nil
}

func (c *Client) Close() error {
	c.dmxDevice.Close()
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

		case tick := <-ticks:
			c.currentStep.Store(int32(tick % 16))

		case <-refresh.C:
			col, _ := c.pattern.ColorAt(int(c.currentStep.Load()))
			c.render(col)

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
		c.pattern.Decode(b[:n])
		c.patternMu.Unlock()
	}
}

func (c *Client) render(col color.Color) {
	rgbaCol, _ := color.RGBAModel.Convert(col).(color.RGBA)

	c.dmxDevice.SetChannel(1, rgbaCol.R)
	c.dmxDevice.SetChannel(2, rgbaCol.G)
	c.dmxDevice.SetChannel(3, rgbaCol.B)

	c.dmxDevice.Render()
}
