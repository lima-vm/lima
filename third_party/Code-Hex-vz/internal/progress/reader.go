package progress

import (
	"io"
	"sync"
	"sync/atomic"
)

// Reader is an io.Reader for checking progress.
type Reader struct {
	once sync.Once

	reader  io.Reader
	total   int64
	current int64
	finish  chan struct{}
	err     error
}

// NewReader create a new io.Reader for checking progress.
func NewReader(rd io.Reader, total, current int64) *Reader {
	return &Reader{
		reader:  rd,
		total:   total,
		current: current,
		finish:  make(chan struct{}),
	}
}

var _ io.Reader = (*Reader)(nil)

// Finish finishes the progress check operation.
func (r *Reader) Finish(err error) {
	r.once.Do(func() {
		r.err = err
		close(r.finish)
	})
}

// Err returns err.
func (r *Reader) Err() error { return r.err }

// Finish sends notification when finished any progress.
func (r *Reader) Finished() <-chan struct{} { return r.finish }

// FractionCompleted returns the fraction of the overall work completed by this progress struct,
// including work done by any children it may have.
func (r *Reader) FractionCompleted() float64 {
	return float64(r.Current()) / float64(r.total)
}

func (r *Reader) Current() int64 {
	return atomic.LoadInt64(&r.current)
}

// Read reads data using underlying io.Reader.
// The number of bytes read is added to the current progress status.
func (r *Reader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	atomic.AddInt64(&r.current, int64(n))
	return
}
