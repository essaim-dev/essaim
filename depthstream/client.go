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
	conn, err := net.ListenUDP("udp4", net.UDPAddrFromAddrPort(addr))
	// conn, err := net.ListenMulticastUDP("udp4", nil, net.UDPAddrFromAddrPort(addr))
	if err != nil {
		return nil, fmt.Errorf("could not listen on multicast address: %w", err)
	}
	conn.SetReadBuffer(binaryImageSize + 1000)

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
			n, _, err := c.conn.ReadFromUDP(b)
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
	// img := image.NewRGBA(image.Rect(0, 0, 640, 480))

	c.binaryImageMu.RLock()
	img := ConvertBitsToRGBA(c.binaryImage, 640, col)
	c.binaryImageMu.RUnlock()

	return flipVertical(img)
}

func binaryImageToRGBA(binaryImage []byte, col color.Color) []byte {
	rgbaCol, _ := color.RGBAModel.Convert(col).(color.RGBA)

	rgba := make([]byte, 0, len(binaryImage)*8*4)

	count := 0
	for _, b := range binaryImage {
		for i := 7; i >= 0; i-- {

			if ((b >> i) & 1) > 0 {
				count++
				rgba = append(rgba, rgbaCol.R, rgbaCol.G, rgbaCol.B, byte(255))
			} else {
				rgba = append(rgba, byte(0), byte(0), byte(0), byte(255))
			}
		}
	}

	return rgba
}

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

// ConvertBitsToRGBA converts a byte slice into an RGBA byte slice.
func ConvertBitsToRGBA(bits []byte, width int, col color.Color) *image.RGBA {
	rgbaCol, _ := color.RGBAModel.Convert(col).(color.RGBA)

	// Determine the height based on the width and total number of bits.
	totalBits := len(bits) * 8
	height := (totalBits + width - 1) / width // Ceiling division

	// Create a new RGBA image.
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	count := 0
	// Loop through each byte and each bit to set RGBA pixels.
	for i, b := range bits {
		for j := 0; j < 8; j++ {
			// Calculate the position of the current bit in the image.
			bitIndex := i*8 + j
			x := bitIndex % width
			y := bitIndex / width

			// Stop if we exceed the total bit count that fits in the image.
			if y >= height {
				break
			}

			// Extract the bit (0 or 1).
			if (b & (1 << (7 - j))) != 0 {
				count++
				img.Set(x, y, rgbaCol) // White for bit 1
			} else {
				img.Set(x, y, color.RGBA{0x00, 0x00, 0x00, 0xFF}) // Black for bit 0
			}
		}
	}

	return img
}
