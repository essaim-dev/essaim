package clock

type Clock interface {
	Tick() <-chan int64
}
