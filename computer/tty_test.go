package computer

// TESTINGTON!!!

import (
	"net"
	"testing"

	"byte-space/utils"
	"github.com/stretchr/testify/assert"
)

type mockProgram struct{}

func (m *mockProgram) SetTTyAPI(api *TTYAPI)                    {}
func (m *mockProgram) SetKernel(api *Kernel)                     {}
func (m *mockProgram) SetProcess(proc *Process)                  {}
func (m *mockProgram) AddGraphicsAPI(api *GraphicsAPI)           {}
func (m *mockProgram) RemoveGraphicsAPI()                        {}
func (m *mockProgram) ID() string                                { return "mock" }
func (m *mockProgram) TTYAPI() *TTYAPI                           { return nil }
func (m *mockProgram) Run(returnStatus chan int, params []string) {}
func (m *mockProgram) HandleSignal(sig Signal)                   {}

func newTestTTY(t *testing.T) (tty *TTY, proc *Process, done chan struct{}) {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() {
		serverConn.Close()
		clientConn.Close()
	})

	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := clientConn.Read(buf); err != nil {
				return
			}
		}
	}()

	eb := NewEventBus()
	tty = NewTTY(serverConn, eb, "tty-test")
	tty.ForegroundPGID = 1

	kernel := &Kernel{procs: map[int]*Process{}}
	tty.Session = &Session{Computer: &Computer{Kernel: kernel}}

	proc = &Process{PGID: 1, Program: &mockProgram{}}
	kernel.procs[1] = proc

	done = make(chan struct{})
	t.Cleanup(func() { close(done) })
	return
}

func typeKeys(tty *TTY, ks ...string) {
	go func() {
		for _, k := range ks {
			tty.dataChannel <- k
		}
	}()
}

func TestCanonicalModeRead(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want string
	}{
		{
			name: "plain text returned on enter",
			keys: []string{"h", "e", "l", "l", "o", "\r"},
			want: "hello",
		},
		{
			name: "empty enter returns empty string",
			keys: []string{"\r"},
			want: "",
		},
		{
			name: "tab is expanded to four spaces",
			keys: []string{"\t", "\r"},
			want: "    ",
		},
		{
			name: "tab expansion inside other text",
			keys: []string{"a", "\t", "b", "\r"},
			want: "a    b",
		},
		{
			name: "arrow up/down are no-ops",
			keys: []string{"a", "\x1b[A", "\x1b[B", "b", "\r"},
			want: "ab",
		},
		{
			name: "unicode runes are buffered correctly",
			keys: []string{"é", "à", "\r"},
			want: "éà",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tty, proc, done := newTestTTY(t)
			typeKeys(tty, tt.keys...)
			s, status := tty.Read(proc, done)
			assert.Equal(t, tt.want, s)
			assert.Equal(t, utils.Success, status)
		})
	}
}

func TestCanonicalModeBackspace(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want string
	}{
		{
			name: "backspace removes last character",
			keys: []string{"a", "b", "c", "\x7f", "\r"},
			want: "ab",
		},
		{
			name: "backspace on empty buffer is a no-op",
			keys: []string{"\x7f", "a", "b", "\r"},
			want: "ab",
		},
		{
			name: "multiple backspaces delete multiple chars",
			keys: []string{"a", "b", "c", "\x7f", "\x7f", "\r"},
			want: "a",
		},
		{
			name: "backspacing all chars leaves empty buffer",
			keys: []string{"a", "b", "c", "\x7f", "\x7f", "\x7f", "\r"},
			want: "",
		},
		{
			name: "extra backspaces past empty are no-ops",
			keys: []string{"a", "\x7f", "\x7f", "\x7f", "b", "\r"},
			want: "b",
		},
		{
			name: "backspace in middle of line removes char left of cursor",
			keys: []string{"a", "b", "c", "\x1b[D", "\x1b[D", "\x7f", "\r"},
			want: "bc",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tty, proc, done := newTestTTY(t)
			typeKeys(tty, tt.keys...)
			s, _ := tty.Read(proc, done)
			assert.Equal(t, tt.want, s)
		})
	}
}

func TestCanonicalModeCursorMovement(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want string
	}{
		{
			name: "arrow left moves cursor enabling mid-line insert",
			keys: []string{"a", "c", "\x1b[D", "b", "\r"},
			want: "abc",
		},
		{
			name: "arrow left at start is a no-op",
			keys: []string{"\x1b[D", "\x1b[D", "a", "b", "\r"},
			want: "ab",
		},
		{
			name: "arrow right at end of buffer is a no-op",
			keys: []string{"a", "b", "\x1b[C", "c", "\r"},
			want: "abc",
		},
		{
			name: "left then right returns cursor to end",
			keys: []string{"a", "c", "\x1b[D", "\x1b[C", "d", "\r"},
			want: "acd",
		},
		{
			name: "move to start then insert prepends",
			keys: []string{"b", "c", "\x1b[D", "\x1b[D", "a", "\r"},
			want: "abc",
		},
		{
			name: "insert at mid-point shifts right half",
			keys: []string{"b", "d", "\x1b[D", "c", "\x1b[D", "\x1b[D", "\x1b[D", "a", "\r"},
			want: "abcd",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tty, proc, done := newTestTTY(t)
			typeKeys(tty, tt.keys...)
			s, _ := tty.Read(proc, done)
			assert.Equal(t, tt.want, s)
		})
	}
}

func TestRawMode(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "letter", key: "x"},
		{name: "digit", key: "3"},
		{name: "space", key: " "},
		{name: "backspace is passed through unmodified", key: "\x7f"},
		{name: "tab is passed through unmodified", key: "\t"},
		{name: "escape sequence is passed through", key: "\x1b[A"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tty, proc, done := newTestTTY(t)
			tty.Canonical = false

			typeKeys(tty, tt.key)
			s, status := tty.Read(proc, done)
			assert.Equal(t, tt.key, s)
			assert.Equal(t, utils.Success, status)
		})
	}
}

func TestReadAccessControl(t *testing.T) {
	t.Run("non-foreground process is rejected immediately", func(t *testing.T) {
		tty, _, done := newTestTTY(t)
		background := &Process{PGID: 99, Program: &mockProgram{}}
		s, status := tty.Read(background, done)
		assert.Equal(t, utils.Error, status)
		assert.Contains(t, s, "not foreground")
	})

	t.Run("closing done channel unblocks Read with SIGINT", func(t *testing.T) {
		tty, proc, _ := newTestTTY(t)
		myDone := make(chan struct{})

		type result struct{ s string; status int }
		ch := make(chan result, 1)
		go func() {
			s, status := tty.Read(proc, myDone)
			ch <- result{s, status}
		}()

		close(myDone)
		res := <-ch
		assert.Equal(t, "SIGINT", res.s)
		assert.Equal(t, utils.Exit, res.status)
	})
}
