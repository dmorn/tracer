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

package tracer_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/tecnoporto/pubsub"
	"github.com/tecnoporto/tracer"
)

type pg struct {
	id         string
	shouldFail bool
}

type addr struct {
}

func (p *pg) Addr() net.Addr {
	return new(addr)
}

func (a *addr) String() string {
	return "host:port"
}

func (a *addr) Network() string {
	return "tcp"
}

func (p *pg) ID() string {
	return p.id
}

func (p *pg) Ping(ctx context.Context) error {
	if p.shouldFail {
		return errors.New("should fail")
	}

	return nil
}

func TestRun(t *testing.T) {
	tr := tracer.New()

	if err := tr.Run(); err != nil {
		t.Fatal(err)
	}
	if tr.Status() != tracer.StatusRunning {
		t.Fatalf("unexpected tracer status: found %v, expected %v", tr.Status(), tracer.StatusRunning)
	}

	if err := tr.Run(); err == nil {
		t.Fatal("tracer should be running")
	}

	tr.Close()
	if tr.Status() != tracer.StatusStopped {
		t.Fatalf("unexpected tracer status: found %v, expected %v", tr.Status(), tracer.StatusStopped)
	}
}

func TestTrace(t *testing.T) {
	tr := tracer.New()
	tr.RefreshRate = time.Millisecond

	if err := tr.Run(); err != nil {
		t.Fatal(err)
	}
	p := &pg{shouldFail: false, id: "fake"} // it looks like the host is up

	if err := tr.Trace(p); err != nil {
		t.Fatal(err)
	}

	wait := make(chan struct{}, 1)
	cancel, err := tr.Sub(&pubsub.Command{
		Topic: tracer.TopicConn,
		Run: func(i interface{}) error {
			m, ok := i.(tracer.Message)
			if !ok {
				t.Fatalf("wrong trace data found: %v", i)
			}

			if m.ID != "fake" {
				t.Fatalf("found wrong id: wanted %v, found %v", "fake", m.ID)
			}

			wait <- struct{}{}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	defer cancel()

	<-wait
}
