package capture

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// CaptureOuts and its Exec() method provide for starting a process
// and then capturing and accessing its output before
// it has completed using BytesSoFar() and GetComboOutSoFar().
//
type CaptureOuts struct {
	lines    []string // segment by lines, so stdout and stderr don't mangle/cross talk.
	isStdErr []bool
	halfline [2]*string // halfline[0] for stdout, halfline[1] for stderr
	mut      sync.Mutex

	wg sync.WaitGroup

	fromChildStdout io.ReadCloser
	fromChildStderr io.ReadCloser

	cmd  *exec.Cmd
	Done chan struct{}
	Err  error
}

func NewCaptureOuts() *CaptureOuts {
	return &CaptureOuts{
		Done: make(chan struct{}),
	}
}

// GetComboOutSoFar can be called by any goroutine at any point to
// obtain the total combined stdout and stderr thus far. Additional
// call make yeild more output, with the same prefix.
//
// If getIsStdErrorSlice is false, the returned isStdErr will be nil.
// Otherwise we fill out the isStdErr slice to correspond, match,
// and inform the res slice.
func (c *CaptureOuts) GetComboOutSoFar(getIsStdErrorSlice bool) (res []string, isStdErr []bool) {
	c.mut.Lock()
	//vv("top of GetComboOutSoFar, c.lines='%#v'", c.lines)
	res = make([]string, len(c.lines))
	copy(res, c.lines)
	if getIsStdErrorSlice {
		isStdErr = make([]bool, len(c.isStdErr))
		copy(isStdErr, c.isStdErr)
	}
	c.mut.Unlock()
	return
}

// BytesSoFar returns both stdout and stderr up
// until this point. Calling again will always return the
// same plus possible additional, newly added, output.
func (c *CaptureOuts) BytesSoFar() []byte {
	var b bytes.Buffer
	for _, v := range c.lines {
		b.WriteString(v)
	}
	return b.Bytes()
}

func (c *CaptureOuts) Exec(arg0 string, args ...string) error {
	cmd := exec.Command(arg0, args...)
	defer close(c.Done)
	c.cmd = cmd

	fromChildStdout, _ := cmd.StdoutPipe()
	fromChildStderr, _ := cmd.StderrPipe()

	c.capture(fromChildStdout, true)
	c.capture(fromChildStderr, false)

	err := cmd.Start()
	if err != nil {
		c.Err = fmt.Errorf("error in CaptureOuts.Exec(): cmd.Start() failed with '%s'", err)
		return c.Err
	}

	// cmd.Wait() should be called only after we finish reading
	// from fromChildStdout and fromChildStderr.
	c.wg.Wait()

	err = cmd.Wait()
	if err != nil {
		c.Err = fmt.Errorf("error in CaptureOuts.Exec(): cmd.Wait() failed with err='%v'", err)
		return c.Err
	}
	return nil
}

func (c *CaptureOuts) capture(r io.Reader, isStdout bool) {
	a := 1 // for stderr
	if isStdout {
		a = 0
	}
	c.wg.Add(1)
	bufreader := bufio.NewReaderSize(r, 1024*1024*8)

	go func() {
		defer c.wg.Done()
		for {
			var err error

			for err == nil {
				// get a fresh line each time, so we can save them without overwriting them.
				line, err2 := bufreader.ReadString('\n') // line will include the newline character.
				//vv("line = '%v'", line)
				err = err2
				if strings.HasSuffix(line, "\n") {
					c.mut.Lock()
					if c.halfline[a] != nil {
						c.lines = append(c.lines, (*c.halfline[a])+line)
						c.isStdErr = append(c.isStdErr, !isStdout)
						c.halfline[a] = nil
					} else {
						c.lines = append(c.lines, line)
						c.isStdErr = append(c.isStdErr, !isStdout)
					}
					//vv("saw full line, c.lines is now '%#v'", c.lines)
					c.mut.Unlock()
				} else {
					if line != "" {
						c.halfline[a] = &line
						//vv("saw half line '%s'", line)
					}
				}
			}
			if c.halfline[a] != nil && *c.halfline[a] != "" {
				c.mut.Lock()
				c.lines = append(c.lines, *(c.halfline[a]))
				c.isStdErr = append(c.isStdErr, !isStdout)
				c.mut.Unlock()
			}
			//vv("before the EOF check, n=%v, c.lines = '%#v', err='%v'", n, c.lines, err)
			if err == io.EOF {
				return
			}
		}
	}()
}

/*
func main() {

	c := NewCaptureOuts()
	go func() {
		err := c.Exec("./slow")
		fmt.Printf("after Exec of slow: err='%v'; final output: ", err)
	}()

	comboLines, _ := c.GetComboOutSoFar(false)
	fmt.Printf("i=%v, comboLines='%v'\n", 0, strings.Join(comboLines, ""))
	for i := 1; i < 5; i++ {
		time.Sleep(time.Second)
		comboLines, _ = c.GetComboOutSoFar(false)
		fmt.Printf("i=%v, comboLines='%v'\n", i, strings.Join(comboLines, ""))
	}
	<-c.Done

	comboLines, _ = c.GetComboOutSoFar(false)
	fmt.Printf("at tm %v, final comboLines='%v'\n", time.Now(), strings.Join(comboLines, ""))
}
*/
