package common

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
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

func (s Service) String() string {
	s.mx.Lock()
	defer s.mx.Unlock()
	return fmt.Sprintf("%s: %d", s.name, s.count)
}

func NewBackpressureMonitor() *BackpressureMonitor {
	return &BackpressureMonitor{
		services: make(map[string]*Service),
	}
}

func (s BackpressureMonitor) Log(level slog.Level) {
	services := make([]string, 0, len(s.services))
	for _, service := range s.services {
		services = append(services, service.String())
	}

	memStats := &runtime.MemStats{}
	runtime.ReadMemStats(memStats)

	slog.Log(context.Background(), level, "backpressure status",
		"services", fmt.Sprintf("[%s]", strings.Join(services, ", ")),
		"goroutines", runtime.NumGoroutine(),
		"heap_sys", mib(memStats.HeapSys),
		"stack_sys", mib(memStats.StackSys),
	)
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

func mib(bytes uint64) float64 {
	return float64(bytes) / 1024 / 1024
}
