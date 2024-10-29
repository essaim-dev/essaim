package clock

import "time"

type FakeClock struct {
}

func NewKafeClock() *FakeClock {
	return &FakeClock{}
}

func (c *FakeClock) Close() error {
	return nil
}

func (c *FakeClock) Tick() <-chan int64 {
	ch := make(chan int64)

	go func() {
		x := int64(0)
		t := time.NewTicker(time.Millisecond * 200)
		for range t.C {
			ch <- x
			x++
		}
	}()

	return ch
}
