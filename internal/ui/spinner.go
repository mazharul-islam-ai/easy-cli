package ui

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"easy-cli/internal/app"
)

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner writes live stage updates to a TTY (or one-line logs to a pipe).
type Spinner struct {
	w     io.Writer
	tty   bool
	mu    sync.Mutex
	stage string
	stop  chan struct{}
	done  chan struct{}
	on    bool
}

// NewSpinner starts a background spinner if w is a TTY file.
func NewSpinner(w io.Writer) *Spinner {
	s := &Spinner{w: w, stop: make(chan struct{}), done: make(chan struct{})}
	if f, ok := w.(*os.File); ok && IsTTY(f) {
		s.tty = true
	}
	return s
}

// Progress implements app.Progress.
func (s *Spinner) Progress(stage app.Stage, detail string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.stage = detail
	s.mu.Unlock()
	if stage == app.StageDone {
		s.Stop()
		return
	}
	if s.tty {
		s.ensureLoop()
		return
	}
	if detail != "" {
		fmt.Fprintf(s.w, "… %s\n", detail)
	}
}

func (s *Spinner) ensureLoop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.on {
		return
	}
	s.on = true
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	go s.loop()
}

func (s *Spinner) loop() {
	defer close(s.done)
	t := time.NewTicker(80 * time.Millisecond)
	defer t.Stop()
	i := 0
	for {
		select {
		case <-s.stop:
			fmt.Fprint(s.w, "\r\033[K")
			return
		case <-t.C:
			s.mu.Lock()
			msg := s.stage
			s.mu.Unlock()
			frame := spinFrames[i%len(spinFrames)]
			i++
			if msg == "" {
				continue
			}
			line := StatusLine(frame, msg)
			fmt.Fprintf(s.w, "\r\033[K%s", line)
		}
	}
}

// Stop clears the spinner line.
func (s *Spinner) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if !s.on {
		s.mu.Unlock()
		return
	}
	s.on = false
	close(s.stop)
	s.mu.Unlock()
	<-s.done
}

// AsProgress returns an app.Progress callback.
func (s *Spinner) AsProgress() app.Progress {
	if s == nil {
		return nil
	}
	return s.Progress
}
