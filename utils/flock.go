package utils

import (
	"os"
	"syscall"
)

type Flock struct {
	name string
	f    *os.File
}

func NewFlock(file string) *Flock {
	return &Flock{
		name: file,
	}
}

func (p *Flock) Lock() error {
	f, err := os.OpenFile(p.name, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}

	p.f = f

	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

func (p *Flock) UnLock() {
	defer p.f.Close()
	syscall.Flock(int(p.f.Fd()), syscall.LOCK_UN)
}
