package mikrocontroller

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"essaim.dev/essaim/clock"
	"essaim.dev/essaim/pattern"
	"essaim.dev/mikro"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	patternsCount = 16
	channelsCount = 4

	controllerRefreshRate = time.Duration(time.Millisecond * 50)
	publishRefreshRate    = time.Duration(time.Second)
)

var (
	padColors = []color.RGBA{
		mikro.ColorOff:         {0, 0, 0, 255},
		mikro.ColorRed:         {255, 0, 0, 255},
		mikro.ColorOrange:      {255, 110, 65, 255},
		mikro.ColorLightOrange: {255, 154, 18, 255},
		mikro.ColorWarmYellow:  {255, 215, 0, 255},
		mikro.ColorYellow:      {255, 255, 0, 255},
		mikro.ColorLime:        {172, 255, 47, 255},
		mikro.ColorGreen:       {0, 255, 0, 255},
		mikro.ColorMint:        {0, 255, 172, 255},
		mikro.ColorCyan:        {0, 255, 255, 255},
		mikro.ColorTurquoise:   {0, 202, 255, 255},
		mikro.ColorBlue:        {0, 48, 255, 255},
		mikro.ColorPlum:        {66, 50, 210, 255},
		mikro.ColorViolet:      {116, 48, 211, 255},
		mikro.ColorPurple:      {211, 50, 245, 255},
		mikro.ColorMagenta:     {255, 0, 255, 255},
		mikro.ColorFuchsia:     {255, 0, 136, 255},
		mikro.ColorWhite:       {255, 255, 255, 255},
	}

	padPalette = color.Palette{}

	modeButtons = map[mikro.Button]PadMode{
		mikro.ButtonPadMode:  PadModeColor,
		mikro.ButtonStep:     PadModeStep,
		mikro.ButtonPattern:  PadModePattern,
		mikro.ButtonKeyboard: PadModeLive,
	}

	selectableColors = map[mikro.Pad]mikro.Color{
		mikro.PadNumber13: mikro.ColorWhite,
		mikro.PadNumber14: mikro.ColorRed,
		mikro.PadNumber15: mikro.ColorOrange,
		mikro.PadNumber16: mikro.ColorLightOrange,

		mikro.PadNumber9:  mikro.ColorWarmYellow,
		mikro.PadNumber10: mikro.ColorYellow,
		mikro.PadNumber11: mikro.ColorLime,
		mikro.PadNumber12: mikro.ColorGreen,

		mikro.PadNumber5: mikro.ColorMint,
		mikro.PadNumber6: mikro.ColorTurquoise,
		mikro.PadNumber7: mikro.ColorBlue,
		mikro.PadNumber8: mikro.ColorPlum,

		mikro.PadNumber1: mikro.ColorViolet,
		mikro.PadNumber2: mikro.ColorPurple,
		mikro.PadNumber3: mikro.ColorMagenta,
		mikro.PadNumber4: mikro.ColorFuchsia,
	}
)

func init() {
	for _, color := range padColors {
		padPalette = append(padPalette, color)
	}
}

type PadMode int

const (
	PadModeColor PadMode = iota + 1
	PadModeStep
	PadModePattern
	PadModeLive
)

type Controller struct {
	device *mikro.Mk3
	clock  clock.Clock
	conn   *net.UDPConn

	activeChannel   atomic.Uint64
	patternChannels [][]*pattern.ColorPattern
	activePattern   atomic.Int32

	mode   PadMode
	modeMu sync.RWMutex

	currentStep atomic.Int32

	picked   mikro.Color
	pickedMu sync.RWMutex

	livePressed   map[mikro.Pad]uint16
	livePressedMu sync.RWMutex
}

func NewController(clock clock.Clock, stepCount int, addr netip.AddrPort) (*Controller, error) {
	dev, err := mikro.OpenMk3()
	if err != nil {
		return nil, fmt.Errorf("could not open mikro device: %w", err)
	}

	conn, err := net.DialUDP("udp4", nil, net.UDPAddrFromAddrPort(addr))
	if err != nil {
		return nil, fmt.Errorf("could not dial udp address: %w", err)
	}
	conn.SetReadBuffer(512)

	c := &Controller{
		clock:           clock,
		device:          dev,
		conn:            conn,
		activeChannel:   atomic.Uint64{},
		patternChannels: make([][]*pattern.ColorPattern, channelsCount),
		mode:            PadModeColor,
		activePattern:   atomic.Int32{},
		currentStep:     atomic.Int32{},
		picked:          mikro.ColorWhite,
		livePressed:     make(map[mikro.Pad]uint16, 16),
	}

	for idx := range c.patternChannels {
		c.patternChannels[idx] = make([]*pattern.ColorPattern, patternsCount)
		for patternIdx := range c.patternChannels[idx] {
			c.patternChannels[idx][patternIdx] = pattern.NewColorPattern(stepCount)
		}
	}

	return c, nil
}

func (c *Controller) Close() error {
	c.conn.Close()
	return c.device.Close()
}

func (c *Controller) Run(ctx context.Context) error {
	c.device.SetOnButtonFunc(c.onButtonPressed)
	c.device.SetOnPadFunc(c.onPadPressed)

	deviceErr := make(chan error, 1)
	go func() {
		deviceErr <- c.device.Run(ctx)
	}()

	ticks := c.clock.Tick()

	refreshController := time.NewTicker(controllerRefreshRate)
	defer refreshController.Stop()
	refreshPublish := time.NewTicker(publishRefreshRate)
	defer refreshPublish.Stop()

	c.updateScreen()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-deviceErr:
			return fmt.Errorf("device stopped running with error: %w", err)

		case tick := <-ticks:
			c.currentStep.Store(int32(tick % 16))

		case <-refreshController.C:
			c.renderController()

		case <-refreshPublish.C:
			if c.activeChannel.Load() == 0 {
				go c.publishActivePattern()
			}
		}
	}
}

func (c *Controller) renderController() error {

	l := c.device.Lights()

	c.setButtonLights(&l)

	switch c.padMode() {
	case PadModeColor:
		c.renderColorModePads(&l)
	case PadModeStep:
		c.renderStepModePads(&l)
	case PadModePattern:
		c.renderPatternModePads(&l)
	case PadModeLive:
		c.renderLiveModePads(&l)
	}

	if err := c.device.SetLights(l); err != nil {
		return fmt.Errorf("could not update controller pad lights: %w", err)
	}

	return nil
}

func (c *Controller) setButtonLights(lights *mikro.Lights) {
	for idx := range lights.Buttons {

		i := mikro.IntensityOff
		mode, ok := modeButtons[mikro.Button(idx)]
		if ok {
			i = mikro.IntensityLow
			if mode == c.padMode() {
				i = mikro.IntensityHigh
			}
		}

		lights.Buttons[idx] = i
	}

	lights.Buttons[mikro.ButtonArrowRight] = mikro.IntensityMedium
	lights.Buttons[mikro.ButtonArrowLeft] = mikro.IntensityMedium
}

func (c *Controller) onPadPressed(msg mikro.PadMessage) {
	switch c.padMode() {
	case PadModeColor:
		c.onPadPressedInButtonMode(msg)
	case PadModeStep:
		c.onPadPressedInStepMode(msg)
		go c.publishActivePattern()
	case PadModePattern:
		c.onPadPressedInPatternMode(msg)
		go c.publishActivePattern()
	case PadModeLive:
		c.onPadPressedInLiveMode(msg)
		go c.publishActivePattern()
	}
}

func (c *Controller) onPadPressedInButtonMode(msg mikro.PadMessage) {
	if msg.Velocity() > 0 || msg.Action() == mikro.PadActionTouched {
		return
	}

	color, ok := selectableColors[msg.Pad()]
	if !ok {
		return
	}

	c.setPickedColor(color)
}

func (c *Controller) onPadPressedInStepMode(msg mikro.PadMessage) {
	if msg.Velocity() > 0 || msg.Action() == mikro.PadActionTouched {
		return
	}

	pattern := c.patternChannels[c.activeChannel.Load()][c.activePattern.Load()]

	patternColor, _ := pattern.ColorAt(int(msg.Pad()))
	padColor := mikro.Color(padPalette.Index(patternColor))

	if padColor == mikro.ColorOff {
		pattern.SetColorAt(int(msg.Pad()), padColors[c.pickedColor()])
	} else {
		pattern.SetColorAt(int(msg.Pad()), padColors[mikro.ColorOff])
	}
}

func (c *Controller) onPadPressedInPatternMode(msg mikro.PadMessage) {
	if msg.Velocity() > 0 || msg.Action() == mikro.PadActionTouched {
		return
	}

	c.activePattern.Store(int32(msg.Pad()))
}

func (c *Controller) onPadPressedInLiveMode(msg mikro.PadMessage) {
	if msg.Action() != mikro.PadActionTouched {
		return
	}

	c.livePressedMu.Lock()
	defer c.livePressedMu.Unlock()
	c.livePressed[msg.Pad()] = msg.Velocity()
}

func (c *Controller) onButtonPressed(msg mikro.ButtonMessage) {
	for _, btn := range msg.PressedButtons() {
		switch btn {
		case mikro.ButtonPadMode:
			c.setPadMode(PadModeColor)
		case mikro.ButtonStep:
			c.setPadMode(PadModeStep)
		case mikro.ButtonPattern:
			c.setPadMode(PadModePattern)
		case mikro.ButtonKeyboard:
			c.setPadMode(PadModeLive)
		case mikro.ButtonArrowRight:
			c.incrementActiveChannel()
			go c.updateScreen()
		case mikro.ButtonArrowLeft:
			c.decrementActiveChannel()
			go c.updateScreen()
		}
	}
}

func (c *Controller) incrementActiveChannel() {
	ch := c.activeChannel.Load()
	if (ch + 1) < channelsCount {
		c.activeChannel.Store(ch + 1)
	}
}

func (c *Controller) decrementActiveChannel() {
	ch := c.activeChannel.Load()
	if ch > 0 {
		c.activeChannel.Store(ch - 1)
	}
}

func (c *Controller) padMode() PadMode {
	c.modeMu.RLock()
	defer c.modeMu.RUnlock()

	return c.mode
}

func (c *Controller) setPadMode(mode PadMode) {
	c.modeMu.Lock()
	defer c.modeMu.Unlock()

	c.mode = mode
}

func (c *Controller) renderColorModePads(lights *mikro.Lights) {
	for idx := range lights.Pads {
		color := mikro.ColorOff
		level := mikro.ColorLevelLow

		if col, ok := selectableColors[mikro.Pad(idx)]; ok {
			color = col
			if color == c.pickedColor() {
				level = mikro.ColorLevelHigh
			}
		}

		lights.Pads[idx] = mikro.ColoredLight{
			Color: color,
			Level: level,
		}
	}
}

func (c *Controller) renderStepModePads(lights *mikro.Lights) {
	pattern := c.patternChannels[c.activeChannel.Load()][c.activePattern.Load()]

	for idx := range lights.Pads {
		level := mikro.ColorLevelHigh
		if idx == int(c.currentStep.Load()) {
			level = mikro.ColorLevelFaded
		}

		col, _ := pattern.ColorAt(idx)
		color := mikro.Color(padPalette.Index(col))
		if color == mikro.ColorOff && idx == int(c.currentStep.Load()) {
			level = mikro.ColorLevelLow
			color = mikro.ColorWhite
		}

		lights.Pads[idx] = mikro.ColoredLight{
			Level: level,
			Color: color,
		}
	}
}

func (c *Controller) renderPatternModePads(lights *mikro.Lights) {
	activePattern := c.activePattern.Load()
	step := c.currentStep.Load()

	for idx := range lights.Pads {
		level := mikro.ColorLevelHigh
		if idx == int(activePattern) {
			level = mikro.ColorLevelFaded
		}

		p := c.patternChannels[c.activeChannel.Load()][idx]

		patternColor, _ := p.ColorAt(int(step))
		padColor := mikro.Color(padPalette.Index(patternColor))
		if idx == int(activePattern) && padColor == mikro.ColorOff {
			level = mikro.ColorLevelLow
			padColor = mikro.ColorWhite
		}

		lights.Pads[idx] = mikro.ColoredLight{
			Color: padColor,
			Level: level,
		}
	}
}

func (c *Controller) renderLiveModePads(lights *mikro.Lights) {
	for idx := range lights.Pads {
		color := mikro.ColorOff
		level := mikro.ColorLevelHigh

		if col, ok := selectableColors[mikro.Pad(idx)]; ok {
			color = col
		}
		c.livePressedMu.RLock()
		if velocity := c.livePressed[mikro.Pad(idx)]; velocity > 0 {
			level = mikro.ColorLevelFaded
		}
		c.livePressedMu.RUnlock()

		lights.Pads[idx] = mikro.ColoredLight{
			Color: color,
			Level: level,
		}
	}
}

func (c *Controller) pickedColor() mikro.Color {
	c.pickedMu.RLock()
	defer c.pickedMu.RUnlock()

	return c.picked
}

func (c *Controller) setPickedColor(color mikro.Color) {
	c.pickedMu.Lock()
	defer c.pickedMu.Unlock()

	c.picked = color
}

func (c *Controller) publishActivePattern() error {
	fmt.Println("write go")
	_, err := c.conn.Write(c.currentPattern().Encode(c.activeChannel.Load()))
	fmt.Println("write done")
	if err != nil {
		return fmt.Errorf("could not write active pattern to udp conn: %w", err)
	}

	return nil
}

func (c *Controller) currentPattern() *pattern.ColorPattern {
	switch c.padMode() {
	case PadModeLive:
		return c.livePattern()
	default:
		return c.patternChannels[c.activeChannel.Load()][c.activePattern.Load()]
	}
}

func (c *Controller) livePattern() *pattern.ColorPattern {
	c.livePressedMu.Lock()
	pressedColors := make([]color.RGBA, 0, len(c.livePressed))
	for idx, velocity := range c.livePressed {
		if velocity > 0 {
			col := padColors[idx]
			col.A = scaleVelocityToUint8(velocity)
			pressedColors = append(pressedColors, col)
		}
	}
	c.livePressedMu.Unlock()

	col := blendRGBAColors(pressedColors)

	p := pattern.NewColorPattern(16)
	for idx := range p.Steps() {
		p.SetColorAt(idx, col)
	}

	return p
}

func (c *Controller) updateScreen() {
	deviceImage := image.NewGray(image.Rect(0, 0, 128, 32))
	point := fixed.Point26_6{
		X: fixed.I(10),
		Y: fixed.I(10),
	}
	fontDrawer := &font.Drawer{
		Dst:  deviceImage,
		Src:  image.White,
		Face: basicfont.Face7x13,
		Dot:  point,
	}
	fontDrawer.DrawString(fmt.Sprintf("chan: %d", c.activeChannel.Load()))
	if err := c.device.SetScreen(deviceImage); err != nil {
		fmt.Printf("could not update device screen: %s\n", err)
	}
}

func scaleVelocityToUint8(value uint16) uint8 {
	if value > 4095 {
		value = 4095
	}

	scaledValue := (value * 255) / 4095

	return uint8(scaledValue)
}

func blendRGBAColors(colors []color.RGBA) color.RGBA {
	if len(colors) == 0 {
		return color.RGBA{0, 0, 0, 0}
	}

	result := color.RGBA{0, 0, 0, 0}

	for _, c := range colors {
		a1 := float64(result.A) / 255
		a2 := float64(c.A) / 255

		aOut := a1 + a2*(1-a1)

		if aOut == 0 {
			continue
		}

		rOut := uint8((float64(result.R)*a1 + float64(c.R)*a2*(1-a1)) / aOut)
		gOut := uint8((float64(result.G)*a1 + float64(c.G)*a2*(1-a1)) / aOut)
		bOut := uint8((float64(result.B)*a1 + float64(c.B)*a2*(1-a1)) / aOut)

		result = color.RGBA{rOut, gOut, bOut, uint8(aOut * 255)}
	}

	return result
}
