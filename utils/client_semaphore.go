package utils

type Semaphore struct {
	permits int
	channel chan int
}


func NewSemaphore(permits int) *Semaphore {
	return &Semaphore{
		channel: make(chan int, permits),
		permits: permits,
	}
}


func (s *Semaphore) Acquire() {
	s.channel <- 0
}


func (s *Semaphore) Release() {
	<- s.channel
}


func (s *Semaphore) AvailablePermits() int {
	return s.permits - len(s.channel)
}
