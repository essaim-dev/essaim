package depthstream

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"net"
	"net/netip"
	"sync"

	"github.com/klauspost/compress/zstd"
)

type Client struct {
	conn *net.UDPConn

	decoder *zstd.Decoder

	binaryImageMu sync.RWMutex
	binaryImage   []byte
}

func NewClient(addr netip.AddrPort) (*Client, error) {
	conn, err := net.ListenMulticastUDP("udp4", nil, net.UDPAddrFromAddrPort(addr))
	if err != nil {
		return nil, fmt.Errorf("could not listen on multicast address: %w", err)
	}
	conn.SetReadBuffer(binaryImageSize)

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, fmt.Errorf("could not create decoder: %w", err)
	}

	return &Client{
		conn:    conn,
		decoder: decoder,
	}, nil
}

func (c *Client) Close() error {
	c.decoder.Close()
	return c.conn.Close()
}

func (c *Client) Run(ctx context.Context) error {
	b := make([]byte, binaryImageSize)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled: %w", ctx.Err())
		default:
			n, err := c.conn.Read(b)
			if err != nil && errors.Is(err, io.EOF) {
				return fmt.Errorf("connection closed: %w", err)
			}
			// decoded, err := c.decoder.DecodeAll(b[:n], make([]byte, 0, binaryImageSize))
			// if err != nil {
			// 	fmt.Printf("could not decole recieved frame: %s", err)
			// 	continue
			// }
			c.binaryImageMu.Lock()
			c.binaryImage = b[:n]
			c.binaryImageMu.Unlock()
		}
	}
}

func (c *Client) RenderImage(col color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))

	c.binaryImageMu.RLock()
	copy(img.Pix, binaryImageToRGBA(c.binaryImage, col))
	c.binaryImageMu.RUnlock()

	return HorizontalFlip(img)
}

func binaryImageToRGBA(binaryImage []byte, col color.Color) []byte {
	rgbaCol, _ := color.RGBAModel.Convert(col).(color.RGBA)

	rgba := make([]byte, 0, len(binaryImage)*8*4)

	for _, b := range binaryImage {
		for i := 7; i >= 0; i-- {
			if (b>>i)&1 == 1 {
				rgba = append(rgba, rgbaCol.R, rgbaCol.G, rgbaCol.B, 255)
			} else {
				rgba = append(rgba, 0, 0, 0, 255)
			}
		}
	}

	return rgba
}

// HorizontalFlip flips the given RGBA image horizontally.
func HorizontalFlip(img *image.RGBA) *image.RGBA {
	// Get the bounds of the image
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Create a new RGBA image to store the result
	flippedImg := image.NewRGBA(bounds)

	// Copy pixels from the original image to the flipped image
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Copy pixel from the original image in reverse x order
			originalPixel := img.At(x, y)
			flippedImg.Set(width-1-x, y, originalPixel)
		}
	}

	return flippedImg
}
