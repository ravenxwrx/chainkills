package common

import (
	"log/slog"
	"sync"
)

var monitor *BackpressureMonitor

func GetBackpressureMonitor() *BackpressureMonitor {
	if monitor == nil {
		monitor = NewBackpressureMonitor()
	}

	return monitor
}

type BackpressureMonitor struct {
	services map[string]*Service
}

type Service struct {
	mx *sync.Mutex

	name  string
	count int
}

func NewService(name string) *Service {
	return &Service{
		mx:    &sync.Mutex{},
		name:  name,
		count: 0,
	}
}

func NewBackpressureMonitor() *BackpressureMonitor {
	return &BackpressureMonitor{
		services: make(map[string]*Service),
	}
}

func (b *BackpressureMonitor) Increase(service string) {
	if _, ok := b.services[service]; !ok {
		b.services[service] = NewService(service)
	}

	b.services[service].mx.Lock()
	b.services[service].count++
	b.services[service].mx.Unlock()

	slog.Debug("increased backpressure", "service", service, "count", b.services[service].count)
}

func (b *BackpressureMonitor) Decrease(service string) {
	if bb, ok := b.services[service]; !ok || bb.count == 0 {
		return
	}

	b.services[service].mx.Lock()
	b.services[service].count--
	b.services[service].mx.Unlock()

	slog.Debug("decreased backpressure", "service", service, "count", b.services[service].count)
}
