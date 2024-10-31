package kinect

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"log"
	"sync"

	"essaim.dev/essaim/freenect"
)

type Renderer struct {
	fctx   *freenect.Context
	device *freenect.Device

	// video   []uint8
	// videoMu sync.RWMutex

	depth   []uint16
	depthMu sync.RWMutex
}

func NewClient() (*Renderer, error) {
	fctx, err := freenect.NewContext()
	if err != nil {
		return nil, fmt.Errorf("could not create freenect context: %w", err)
	}

	device, err := fctx.OpenDevice(0)
	if err != nil {
		return nil, fmt.Errorf("could not create freenect context: %w", err)
	}

	device.SetLED(freenect.LEDColorBlinkGreen)

	return &Renderer{
		fctx:   &fctx,
		device: &device,
	}, nil
}

func (r *Renderer) Close() error {
	r.device.SetLED(freenect.LEDColorRed)
	r.device.Destroy()
	return r.fctx.Destroy()
}

// func (r *Renderer) videoFunc(device *freenect.Device, video []byte, timestamp uint32) {
// 	r.videoMu.Lock()
// 	defer r.videoMu.Unlock()

// 	r.video = video
// }

func (r *Renderer) depthFunc(device *freenect.Device, depth []uint16, timestamp uint32) {
	r.depthMu.Lock()
	defer r.depthMu.Unlock()

	r.depth = depth
}

func (r *Renderer) Run(ctx context.Context) error {
	// r.device.SetVideoCallback(r.videoFunc)
	r.device.SetDepthCallback(r.depthFunc)

	// if err := r.device.StartVideoStream(freenect.ResolutionMedium, freenect.VideoFormatRGB); err != nil {
	// 	return fmt.Errorf("could not start video stream: %w", err)
	// }
	// defer r.device.StopVideoStream()

	if err := r.device.StartDepthStream(freenect.ResolutionMedium, freenect.DepthFormatMM); err != nil {
		return fmt.Errorf("could not start video stream: %w", err)
	}
	defer r.device.StopDepthStream()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if err := r.fctx.ProcessEvents(0); err != nil {
				return fmt.Errorf("could not process events: %w", err)
			}
		}

	}
}

func (r *Renderer) RenderImage(c color.Color) *image.RGBA {
	// img := image.NewRGBA(image.Rect(0, 0, 640, 480))

	// r.videoMu.RLock()
	// copy(img.Pix, rgbToRGBA(r.video))
	// r.videoMu.RUnlock()

	depthImg := image.NewRGBA(image.Rect(0, 0, 640, 480))
	r.depthMu.RLock()
	copy(depthImg.Pix, depthToRGBA(r.depth, c))
	r.depthMu.RUnlock()

	// draw.Draw(img, depthImg.Bounds(), depthImg, image.Point{}, draw.Over)

	return flipVertical(depthImg)
}

func rgbToRGBA(rgb []byte) []byte {
	if len(rgb)%3 != 0 {
		log.Fatal("The length of the RGB array must be a multiple of 3.")
	}

	rgba := make([]byte, 0, (len(rgb)/3)*4)

	for i := 0; i < len(rgb); i += 3 {
		rgba = append(rgba, rgb[i], rgb[i+1], rgb[i+2], 255)
	}

	return rgba
}

func depthToRGBA(depth []uint16, c color.Color) []byte {
	rgbaCol, _ := color.RGBAModel.Convert(c).(color.RGBA)

	rgba := make([]byte, 0, (len(depth)))
	for _, d := range depth {
		scaledDepth := scaleTo255(d)
		if scaledDepth == 0 || isBlack(rgbaCol) {
			rgba = append(rgba, byte(0), byte(0), byte(0), byte(0))
		} else {
			rgba = append(rgba, rgbaCol.R, rgbaCol.G, rgbaCol.B, 255)
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

func scaleTo255(value uint16) uint8 {

	// if value > 1000 {
	// 	value = 5000
	// }

	// val := uint64(value)

	// // Scale the value from 0-5000 to 0-255
	// scaledValue := uint8(((val * 255) / 5000))
	// return scaledValue

	if value < 1000 {
		return uint8(((value * 255) / 1000))
	}

	return 0
}

func isBlack(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r == 0 && g == 0 && b == 0
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
