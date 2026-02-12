package downstream

import "encoding/json"

// request represents a JSON-RPC request to send to a downstream process.
type request struct {
	ID     int
	Method string
	Params json.RawMessage // raw JSON-RPC params
	Result chan response
}

// response is the result of a downstream tool call.
type response struct {
	Data json.RawMessage
	Err  error
}

// requestQueue is a buffered channel of pending requests.
type requestQueue struct {
	ch chan request
}

func newRequestQueue(size int) *requestQueue {
	return &requestQueue{ch: make(chan request, size)}
}

func (q *requestQueue) enqueue(r request) {
	q.ch <- r
}

func (q *requestQueue) dequeue() (request, bool) {
	r, ok := <-q.ch
	return r, ok
}

func (q *requestQueue) close() {
	close(q.ch)
}
