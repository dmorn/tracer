/*
MIT License

Copyright (c) 2018 Daniel Morandini

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

// Package tracer provides basic functionalities to monitor a network address
// until it is online.
package tracer

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/tecnoporto/pubsub"
)

// Topic used to publish connectin discovery messgages.
const (
	TopicConn = "topic_connection"
)

// Possible Tracer status value.
const (
	StatusRunning = iota
	StatusStopped
)

// Possible connection states.
const (
	ConnOnline = iota
	ConnOffline
)

// Pinger wraps the basic Ping function.
type Pinger interface {
	Addr() net.Addr
	Ping(ctx context.Context) error
	ID() string
}

// PubSub describes the required functionalities of a publication/subscription object.
type PubSub interface {
	Sub(cmd *pubsub.Command) (pubsub.CancelFunc, error)
	Pub(message interface{}, topic string)
}

// Tracer can monitor remote interfaces until they're up.
type Tracer struct {
	PubSub

	refreshc    chan struct{}
	stopc       chan struct{}
	conns       map[string]Pinger
	RefreshRate time.Duration

	sync.Mutex
	status int
}

type Message struct {
	ID  string
	Err error
}

// New returns a new instance of Tracer.
func New() *Tracer {
	t := &Tracer{
		PubSub:      pubsub.New(),
		conns:       make(map[string]Pinger),
		refreshc:    make(chan struct{}),
		stopc:       make(chan struct{}),
		status:      StatusStopped,
		RefreshRate: time.Second * 4,
	}

	return t
}

// Run makes the tracer listen for refresh calls and perform ping operations
// on each connection that is labeled with pending.
// Quits immediately when Close is called, runs in its own gorountine.
func (t *Tracer) Run() error {
	if t.Status() == StatusRunning {
		return errors.New("tracer: already running")
	}
	t.setStatus(StatusRunning)

	ping := func() context.CancelFunc {
		ctx, cancel := context.WithCancel(context.Background())

		for _, c := range t.conns {
			go func(c Pinger) {
				err := c.Ping(ctx)
				m := Message{ID: c.ID(), Err: err}

				if t.PubSub != nil {
					t.Pub(m, TopicConn)
				}
			}(c)
		}

		return cancel
	}

	go func() {
		var cancel context.CancelFunc
		for {
			refresh := func() {
				if cancel != nil {
					cancel()
				}
				cancel = ping()
			}

			select {
			case <-t.refreshc:
				refresh()
			case <-t.stopc:
				if cancel != nil {
					cancel()
				}
				return
			case <-time.After(t.RefreshRate):
				refresh()
			}
		}
	}()

	return nil
}

// Trace makes the tracer keep track of the entity at addr.
func (t *Tracer) Trace(p Pinger) error {
	t.conns[p.ID()] = p
	t.refresh()

	return nil
}

// Status returns the status of tracer.
func (t *Tracer) Status() int {
	t.Lock()
	defer t.Unlock()
	return t.status
}

func (t *Tracer) setStatus(status int) {
	t.Lock()
	defer t.Unlock()
	t.status = status
}

// Untrace removes the entity stored with id from the monitored
// entities.
func (t *Tracer) Untrace(id string) {
	delete(t.conns, id)
	t.refresh()
}

func (t *Tracer) refresh() {
	t.refreshc <- struct{}{}
}

// Close makes the tracer pass from status running to status stopped.
func (t *Tracer) Close() {
	t.setStatus(StatusStopped)
	t.stopc <- struct{}{}
}
