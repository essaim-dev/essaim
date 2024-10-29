package clock

import (
	"sync"
	"time"

	"github.com/antoi-ne/al"
)

type LinkClock struct {
	link *al.Link

	bpm   float64
	bpmMu sync.RWMutex
}

func NewLinkClock(bpm float64) *LinkClock {
	l := al.NewLink(bpm)

	return &LinkClock{
		bpm:  bpm,
		link: l,
	}
}

func (c *LinkClock) Start() {
	c.link.Enable(true)
}

func (c *LinkClock) Close() error {
	c.link.Enable(false)
	c.link.Close()
	return nil
}

func (c *LinkClock) Tick() <-chan int64 {
	ch := make(chan int64)
	go c.produce(ch)

	return ch
}

func (c *LinkClock) BPM() float64 {
	c.bpmMu.RLock()
	defer c.bpmMu.RUnlock()

	return c.bpm
}

func (c *LinkClock) produce(ch chan int64) {
	state := al.NewSessionState()

	lastStep := int64(0)

	for {
		c.link.CaptureAppSessionState(state)

		beat := state.BeatAtTime(c.link.Clock(), 4)
		step := int64(beat * 4)

		if step > lastStep {
			ch <- step
		}

		lastStep = step

		time.Sleep(time.Millisecond * 10)
	}
}

// func (c *LinkClock) tempoCallback(tempo float64) {
// 	c.bpmMu.Lock()
// 	defer c.bpmMu.Unlock()

// 	c.bpm = tempo
// }
