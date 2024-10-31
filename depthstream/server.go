package depthstream

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"essaim.dev/essaim/freenect"
	"github.com/klauspost/compress/zstd"
)

const (
	binaryImageSize = 640 * 480 / 8
)

type Server struct {
	conn *net.UDPConn

	kinectCtx    *freenect.Context
	kinectDevice *freenect.Device

	depthThresholdMu sync.RWMutex
	depthThreshold   uint16

	encoder *zstd.Encoder
}

func NewServer(addr netip.AddrPort) (*Server, error) {
	conn, err := net.DialUDP("udp4", nil, net.UDPAddrFromAddrPort(addr))
	if err != nil {
		return nil, fmt.Errorf("could not dial udp address: %w", err)
	}
	conn.SetWriteBuffer(binaryImageSize + 100)

	fctx, err := freenect.NewContext()
	if err != nil {
		return nil, fmt.Errorf("could not create freenect context: %w", err)
	}

	device, err := fctx.OpenDevice(0)
	if err != nil {
		return nil, fmt.Errorf("could not create freenect context: %w", err)
	}

	if err := device.SetLED(freenect.LEDColorYellow); err != nil {
		return nil, fmt.Errorf("could not set kinect led: %w", err)
	}

	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest))
	if err != nil {
		return nil, fmt.Errorf("could not create encoder: %w", err)
	}

	return &Server{
		conn:           conn,
		kinectCtx:      &fctx,
		kinectDevice:   &device,
		depthThreshold: 2000,
		encoder:        encoder,
	}, nil
}

func (s *Server) Close() error {
	s.encoder.Close()
	return s.conn.Close()
}

func (s *Server) SetDepthThreshold(threshold uint16) {
	s.depthThresholdMu.Lock()
	defer s.depthThresholdMu.Unlock()

	s.depthThreshold = threshold
}

func (s *Server) Run(ctx context.Context) error {
	s.kinectDevice.SetDepthCallback(s.depthCallback)

	if err := s.kinectDevice.SetLED(freenect.LEDColorBlinkGreen); err != nil {
		return fmt.Errorf("could not set kinect led: %w", err)
	}

	if err := s.kinectDevice.StartDepthStream(freenect.ResolutionMedium, freenect.DepthFormatMM); err != nil {
		return fmt.Errorf("could not start kinect depth stream: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled: %w", ctx.Err())
		default:
			// Do not handle errors, they are most likely timeouts.
			s.kinectCtx.ProcessEvents(time.Millisecond * 500)
		}
	}
}

func (s *Server) depthCallback(device *freenect.Device, depth []uint16, timestamp uint32) {
	binaryImage := s.depthToBinaryImage(depth)

	// encoded := s.encoder.EncodeAll(binaryImage, make([]byte, 0, len(binaryImage)))

	if n, err := s.conn.Write(binaryImage); err != nil {
		fmt.Println(n, err)
	}
}

func (s *Server) depthToBinaryImage(depth []uint16) []byte {
	output := make([]byte, (len(depth)+7)/8)

	s.depthThresholdMu.RLock()
	defer s.depthThresholdMu.RUnlock()

	count := 0
	for idx, value := range depth {
		if value > s.depthThreshold {
			continue // Depth is further than threshold, pixel stays off.
		}
		count++
		output[idx/8] |= (1 << idx % 8)
	}

	fmt.Println("count ", count)

	return output
}
