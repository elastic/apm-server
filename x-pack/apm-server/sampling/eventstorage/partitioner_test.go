// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"fmt"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ringStep struct {
	current  int
	inactive int
	active   []int
}

func TestPartitioner_Ring(t *testing.T) {
	tests := []struct {
		name      string
		actives   int
		current   int
		active    []int
		rotations []ringStep
	}{
		{
			name:    "two active starting at 0",
			actives: 2,
			current: 0,
			active:  []int{0, 2},
			rotations: []ringStep{
				{current: 1, inactive: 2, active: []int{1, 0}},
				{current: 2, inactive: 0, active: []int{2, 1}},
				{current: 0, inactive: 1, active: []int{0, 2}},
			},
		},
		{
			name:    "two active starting at 1",
			actives: 2,
			current: 1,
			active:  []int{1, 0},
		},
		{
			name:    "one active",
			actives: 1,
			current: 0,
			active:  []int{0},
			rotations: []ringStep{
				{current: 1, inactive: 0, active: []int{1}},
				{current: 0, inactive: 1, active: []int{0}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPartitioner(tt.actives, tt.current)
			assertCurrent(t, p, tt.current)
			assert.Equal(t, tt.active, slices.Collect(p.ActiveIDs()))

			for _, step := range tt.rotations {
				newCurrent, newInactive := p.Rotate()
				p.WaitInactive(newInactive)
				assert.Equal(t, step.current, newCurrent)
				assert.Equal(t, step.inactive, newInactive)
				assertCurrent(t, p, step.current)
				assert.Equal(t, step.active, slices.Collect(p.ActiveIDs()))
			}
		})
	}
}

func TestPartitioner_RotateReturnsWhilePinned(t *testing.T) {
	// Rotate must return before WaitInactive, so the caller can persist the new
	// current ID while a reader of the old ring is still running.
	tests := []struct {
		name    string
		pin     func(p *Partitioner, entered chan<- struct{}, release <-chan struct{})
		blocked bool
	}{
		{
			name: "active ids blocks the outgoing partition",
			pin: func(p *Partitioner, entered chan<- struct{}, release <-chan struct{}) {
				var once sync.Once
				for range p.ActiveIDs() {
					once.Do(func() {
						close(entered)
						<-release
					})
				}
			},
			blocked: true,
		},
		{
			name: "current write does not block the first inactive partition",
			pin: func(p *Partitioner, entered chan<- struct{}, release <-chan struct{}) {
				p.CurrentIDFunc(func(int) {
					close(entered)
					<-release
				})
			},
			blocked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPartitioner(2, 0)
			entered := make(chan struct{})
			release := make(chan struct{})
			pinDone := make(chan struct{})
			go func() {
				defer close(pinDone)
				tt.pin(p, entered, release)
			}()

			<-entered
			rotateReturned := make(chan int, 1)
			go func() {
				_, inactive := p.Rotate()
				rotateReturned <- inactive
			}()

			var inactive int
			select {
			case inactive = <-rotateReturned:
			case <-time.After(time.Second):
				t.Fatal("Rotate blocked while a partition was pinned")
			}
			// The new current partition is visible before the outgoing pin is dropped.
			assertCurrent(t, p, 1)

			waitReturned := make(chan struct{})
			go func() {
				p.WaitInactive(inactive)
				close(waitReturned)
			}()

			if tt.blocked {
				select {
				case <-waitReturned:
					t.Fatal("WaitInactive returned while the pin was held")
				case <-time.After(50 * time.Millisecond):
				}
			} else {
				select {
				case <-waitReturned:
				case <-time.After(time.Second):
					t.Fatal("WaitInactive blocked on a pin it does not need")
				}
			}

			close(release)
			select {
			case <-waitReturned:
			case <-time.After(time.Second):
				t.Fatal("WaitInactive still blocked after the pin was released")
			}
			<-pinDone
		})
	}
}

func TestPartitioner_WritePinBlocksPartitionRecycle(t *testing.T) {
	// A write pins current. The next rotation inactivates the oldest partition,
	// not current. The rotation after that inactivates the pinned partition and
	// must wait.
	p := NewPartitioner(2, 0)
	entered := make(chan struct{})
	release := make(chan struct{})
	go func() {
		p.CurrentIDFunc(func(pid int) {
			assert.Equal(t, 0, pid)
			close(entered)
			<-release
		})
	}()
	<-entered

	_, inactive := p.Rotate()
	assert.Equal(t, 2, inactive)
	waited := make(chan struct{})
	go func() {
		p.WaitInactive(inactive)
		close(waited)
	}()
	select {
	case <-waited:
	case <-time.After(time.Second):
		t.Fatal("first WaitInactive blocked on a write to current")
	}

	_, inactive = p.Rotate()
	assert.Equal(t, 0, inactive)
	second := make(chan struct{})
	go func() {
		p.WaitInactive(inactive)
		close(second)
	}()
	select {
	case <-second:
		t.Fatal("second WaitInactive returned while the write pin was held")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-second:
	case <-time.After(time.Second):
		t.Fatal("second WaitInactive still blocked after the write returned")
	}
}

func TestPartitioner_BreakDropsPins(t *testing.T) {
	p := NewPartitioner(2, 0)
	for pid := range p.ActiveIDs() {
		assert.Equal(t, 0, pid)
		break
	}
	_, inactive := p.Rotate()
	waited := make(chan struct{})
	go func() {
		p.WaitInactive(inactive)
		close(waited)
	}()
	select {
	case <-waited:
	case <-time.After(time.Second):
		t.Fatal("WaitInactive blocked after the iterator stopped early")
	}
}

func TestPartitioner_NestedReadDuringWait(t *testing.T) {
	// The production deadlock: a read holds a pin, rotation is waiting on that
	// pin, and the read calls back into ActiveIDs (remote-decision delete).
	// The nested read must run while WaitInactive is registered as a waiter.
	p := NewPartitioner(2, 0)
	entered := make(chan struct{})
	waitDone := make(chan struct{})
	inactiveCh := make(chan int, 1)

	go func() {
		<-entered
		_, inactive := p.Rotate()
		inactiveCh <- inactive
		p.WaitInactive(inactive)
		close(waitDone)
	}()

	var once sync.Once
	for range p.ActiveIDs() {
		once.Do(func() {
			close(entered)
			var inactive int
			select {
			case inactive = <-inactiveCh:
			case <-time.After(time.Second):
				t.Fatal("Rotate blocked inside ActiveIDs")
			}
			// The outer pin keeps WaitInactive from returning, so once the
			// waiter is counted the nested read below races a live waiter.
			deadline := time.Now().Add(time.Second)
			for p.waiters[inactive].Load() == 0 {
				if time.Now().After(deadline) {
					t.Fatal("WaitInactive never registered as a waiter")
				}
				runtime.Gosched()
			}
			select {
			case <-waitDone:
				t.Fatal("WaitInactive returned while the outer read was pinned")
			default:
			}

			nested := make(chan struct{})
			go func() {
				defer close(nested)
				for range p.ActiveIDs() {
				}
				p.CurrentIDFunc(func(int) {})
			}()
			select {
			case <-nested:
			case <-time.After(time.Second):
				t.Fatal("nested read blocked during WaitInactive")
			}
		})
	}

	select {
	case <-waitDone:
	case <-time.After(time.Second):
		t.Fatal("WaitInactive blocked after the outer iteration returned")
	}
}

func TestPartitioner_ReleaseIgnoresOtherPartitions(t *testing.T) {
	// A release that empties a partition nobody waits for must not take waitMu,
	// so intake does not contend with, or wake, a WaitInactive on another partition.
	p := NewPartitioner(2, 0)
	entered := make(chan struct{})
	release := make(chan struct{})
	pinDone := make(chan struct{})
	go func() {
		defer close(pinDone)
		var once sync.Once
		for range p.ActiveIDs() {
			once.Do(func() {
				close(entered)
				<-release
			})
		}
	}()
	<-entered
	_, inactive := p.Rotate()
	waitDone := make(chan struct{})
	go func() {
		p.WaitInactive(inactive)
		close(waitDone)
	}()
	for p.waiters[inactive].Load() == 0 {
		runtime.Gosched()
	}

	// waitMu is free while WaitInactive sleeps. Holding it turns an unwanted
	// broadcast attempt in release into a block.
	p.waitMu.Lock()
	other := make(chan struct{})
	go func() {
		p.CurrentIDFunc(func(int) {}) // current is not the partition being waited on
		close(other)
	}()
	select {
	case <-other:
	case <-time.After(time.Second):
		t.Error("release of an unrelated partition contended on waitMu")
	}
	p.waitMu.Unlock()

	close(release)
	select {
	case <-waitDone:
	case <-time.After(time.Second):
		t.Fatal("WaitInactive still blocked after the pin was released")
	}
	<-pinDone
}

func TestPartitioner_NoLostWakeup(t *testing.T) {
	// Races the last release of a pinned partition against WaitInactive, so every
	// interleaving of the refs check, the waiters count and the broadcast is hit
	// over many iterations. A lost wake-up leaves WaitInactive blocked forever.
	for i := range 20000 {
		p := NewPartitioner(2, 0)
		entered := make(chan struct{})
		release := make(chan struct{})
		go func() {
			var once sync.Once
			for range p.ActiveIDs() {
				once.Do(func() {
					close(entered)
					<-release
				})
			}
		}()
		<-entered
		_, inactive := p.Rotate()
		waitDone := make(chan struct{})
		go func() {
			p.WaitInactive(inactive)
			close(waitDone)
		}()
		if i%2 == 0 {
			runtime.Gosched()
		}
		close(release)
		select {
		case <-waitDone:
		case <-time.After(5 * time.Second):
			t.Fatalf("iteration %d: WaitInactive missed the wake-up", i)
		}
	}
}

func TestPartitioner_HotPathAllocs(t *testing.T) {
	p := NewPartitioner(2, 0)
	active := testing.AllocsPerRun(200, func() {
		for pid := range p.ActiveIDs() {
			if pid < 0 {
				return
			}
		}
	})
	current := testing.AllocsPerRun(200, func() {
		p.CurrentIDFunc(func(pid int) {
			if pid < 0 {
				return
			}
		})
	})
	// The ID snapshot is a stack array, and neither call allocates.
	assert.Zero(t, active)
	assert.Zero(t, current)
}

func assertCurrent(t *testing.T, p *Partitioner, want int) {
	t.Helper()
	p.CurrentIDFunc(func(pid int) {
		assert.Equal(t, want, pid)
	})
}

func TestPartitioner_NewRejectsTooManyActives(t *testing.T) {
	require.Panics(t, func() {
		NewPartitioner(partitionRingSize, 0)
	})
}

// TestPartitioner_ConcurrentStress runs readers, writers and a rotator together.
// The rotator marks a partition cleared once WaitInactive returns, standing in
// for the DeleteRange that follows in StorageManager.RotatePartitions, so a
// reader or writer observing a cleared partition while pinned fails the test.
func TestPartitioner_ConcurrentStress(t *testing.T) {
	const rotations = 300
	p := NewPartitioner(2, 0)

	cleared := make([]atomic.Bool, p.total)
	cleared[1].Store(true) // the initial inactive partition
	check := func(pid int) {
		if cleared[pid].Load() {
			t.Errorf("partition %d was cleared while pinned", pid)
		}
	}

	var stop atomic.Bool
	var readers sync.WaitGroup
	run := func(f func()) {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for !stop.Load() {
				f()
			}
		}()
	}
	for range 2 {
		run(func() { // plain reader
			for pid := range p.ActiveIDs() {
				check(pid)
				runtime.Gosched()
				check(pid)
			}
		})
		run(func() { // reader that stops early
			for pid := range p.ActiveIDs() {
				check(pid)
				break
			}
		})
		run(func() { // reader that calls back in, like the remote-decision delete
			for pid := range p.ActiveIDs() {
				check(pid)
				for inner := range p.ActiveIDs() {
					check(inner)
				}
				p.CurrentIDFunc(check)
				check(pid)
			}
		})
		run(func() { // writer
			p.CurrentIDFunc(func(pid int) {
				check(pid)
				runtime.Gosched()
				check(pid)
			})
		})
	}

	var rotated atomic.Int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		current := 0
		for range rotations {
			// The inactive partition is about to become current again.
			cleared[(current+1)%p.total].Store(false)
			newCurrent, newInactive := p.Rotate()
			current = newCurrent
			p.WaitInactive(newInactive)
			if n := p.refs[newInactive].Load(); n != 0 {
				t.Errorf("partition %d has %d pins after WaitInactive", newInactive, n)
			}
			cleared[newInactive].Store(true)
			rotated.Add(1)
		}
		stop.Store(true)
		readers.Wait()
	}()
	// Rotations wait for descheduled readers, so their pace depends on the
	// machine. Only a long stretch without progress counts as a deadlock.
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	last := int64(-1)
wait:
	for {
		select {
		case <-done:
			break wait
		case <-ticker.C:
			if n := rotated.Load(); n == last {
				t.Fatalf("deadlock: no rotation completed in 10s (%d done)", n)
			} else {
				last = n
			}
		}
	}

	for id := range p.refs {
		assert.Zero(t, p.refs[id].Load(), "partition %d", id)
	}
}

// BenchmarkPartitioner measures the intake hot path (ActiveIDs for reads,
// CurrentIDFunc for writes) under parallel load, with and without a rotation
// every millisecond.
func BenchmarkPartitioner(b *testing.B) {
	calls := map[string]func(p *Partitioner){
		"ActiveIDs": func(p *Partitioner) {
			for pid := range p.ActiveIDs() {
				_ = pid
			}
		},
		"CurrentIDFunc": func(p *Partitioner) {
			p.CurrentIDFunc(func(int) {})
		},
	}
	for name, call := range calls {
		for _, rotate := range []bool{false, true} {
			b.Run(fmt.Sprintf("%s/rotate=%t", name, rotate), func(b *testing.B) {
				p := NewPartitioner(2, 0)
				if rotate {
					stop := make(chan struct{})
					defer close(stop)
					go func() {
						ticker := time.NewTicker(time.Millisecond)
						defer ticker.Stop()
						for {
							select {
							case <-stop:
								return
							case <-ticker.C:
								p.Rotate()
							}
						}
					}()
				}
				b.ReportAllocs()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						call(p)
					}
				})
			})
		}
	}
}
