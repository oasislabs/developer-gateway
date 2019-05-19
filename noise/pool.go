package noise

import (
	"context"
	"io"
)

type request struct {
	Context context.Context
	Reader  io.Reader
	Writer  io.Writer
	Error   chan<- error
}

// FixedConnPool manages a fixed pool of connections and distributes work amongst
// them so that the caller does not need to worry about concurrency
type FixedConnPool struct {
	c chan request
}

// FixedConnPoolProps sets up the connection pool
type FixedConnPoolProps struct {
	Conns        int
	Channel      Channel
	SessionProps SessionProps
}

// DialFixedPool creates a new pool of connections
func DialFixedPool(ctx context.Context, props FixedConnPoolProps) (*FixedConnPool, error) {
	ctx, cancel := context.WithCancel(ctx)
	pool := &FixedConnPool{c: make(chan request, 64)}

	for i := 0; i < props.Conns; i++ {
		// TODO(stan): this can be done in parallel
		if err := pool.dialConnection(ctx, props.Channel, &props.SessionProps); err != nil {
			cancel()
			return nil, err
		}
	}

	return pool, nil
}

// Request issues a request to one of the connections in the pool and
// retrieves the response. The pool is concurrency safe.
func (p *FixedConnPool) Request(ctx context.Context, res io.Writer, req io.Reader) error {
	err := make(chan error)
	p.c <- request{Context: ctx, Reader: req, Writer: res, Error: err}
	return <-err
}

func startConnLoop(ctx context.Context, conn *Conn, c <-chan request) {
	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-c:
			if !ok {
				return
			}

			req.Error <- conn.Request(req.Context, req.Writer, req.Reader)
		}
	}
}

func (p *FixedConnPool) dialConnection(ctx context.Context, channel Channel, props *SessionProps) error {
	conn, err := DialContext(ctx, channel, props)
	if err != nil {
		// TODO(stan): if a connection fails to establish we should shutdown
		// all the successful connection gracefully
		return err
	}

	go startConnLoop(ctx, conn, p.c)
	return nil
}
