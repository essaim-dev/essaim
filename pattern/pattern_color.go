package pattern

import (
	"image/color"
	"sync"

	"essaim.dev/essaim/api/essaimbp"
)

type ColorPattern struct {
	steps   []color.RGBA
	stepsMu sync.RWMutex
}

func NewColorPattern(steps int) *ColorPattern {
	p := &ColorPattern{
		steps: make([]color.RGBA, steps),
	}

	for idx := range p.steps {
		p.steps[idx] = color.RGBA{0, 0, 0, 255}
	}

	return p
}

func (p *ColorPattern) Steps() []color.RGBA {
	p.stepsMu.RLock()
	defer p.stepsMu.RUnlock()

	return p.steps
}

func (p *ColorPattern) ColorAt(step int) (color.Color, bool) {
	p.stepsMu.RLock()
	defer p.stepsMu.RUnlock()

	if step < 0 || step >= len(p.steps) {
		return color.Transparent, false
	}

	return p.steps[step], true
}

func (p *ColorPattern) SetColorAt(step int, c color.RGBA) bool {
	p.stepsMu.Lock()
	defer p.stepsMu.Unlock()

	if step < 0 || step >= len(p.steps) {
		return false
	}

	p.steps[step] = c
	return true
}

func (p *ColorPattern) Encode(ch uint64) []byte {
	p.stepsMu.RLock()
	defer p.stepsMu.RUnlock()

	message := essaimbp.Pattern{
		Channel: ch,
	}

	for idx, stepColor := range p.steps {
		message.Steps[idx] = essaimbp.RGBA{
			R: stepColor.R,
			G: stepColor.G,
			B: stepColor.B,
			A: stepColor.A,
		}
	}

	return message.Encode()
}

func (p *ColorPattern) Decode(b []byte, ch uint64) {
	message := essaimbp.Pattern{}
	message.Decode(b)

	if message.Channel != 0 && message.Channel != ch {
		return
	}

	p.stepsMu.RLock()
	defer p.stepsMu.RUnlock()

	for idx := range p.steps {
		p.steps[idx] = color.RGBA{
			R: message.Steps[idx].R,
			G: message.Steps[idx].G,
			B: message.Steps[idx].B,
			A: message.Steps[idx].A,
		}
	}
}
