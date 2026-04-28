package pending

import "sync"

type Status string

const (
	StatusOK       Status = "ok"
	StatusNotFound Status = "not_found"
	StatusError    Status = "error"
)

type Result struct {
	Status Status
	Err    string
}

var Channels = struct {
	sync.Mutex
	m map[string]chan Result
}{m: make(map[string]chan Result)}

func Register(id string) chan Result {
	ch := make(chan Result, 1)
	Channels.Lock()
	Channels.m[id] = ch
	Channels.Unlock()
	return ch
}

func Completed(id string, r Result) {
	Channels.Lock()
	ch, ok := Channels.m[id]
	if ok {
		ch <- r
		close(ch)
		delete(Channels.m, id)
	}
	Channels.Unlock()
}
