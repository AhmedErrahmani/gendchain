package cmdtest

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"text/template"
	"time"

	"github.com/docker/docker/pkg/reexec"
)

func NewTestCmd(t *testing.T, data interface{}) *TestCmd {
	return &TestCmd{T: t, Data: data}
}

type TestCmd struct {
	// For total convenience, all testing methods are available.
	*testing.T

	Func    template.FuncMap
	Data    interface{}
	Cleanup func()

	cmd    *exec.Cmd
	stdout *bufio.Reader
	stdin  io.WriteCloser
	stderr *testlogger
	// Err will contain the process exit error or interrupt signal error
	Err error
}

// Run exec's the current binary using name as argv[0] which will trigger the
// reexec init function for that name (e.g. "gendchain-test" in cmd/gendchain/run_test.go)
func (tt *TestCmd) Run(name string, args ...string) {
	tt.stderr = &testlogger{t: tt.T}
	tt.cmd = &exec.Cmd{
		Path:   reexec.Self(),
		Args:   append([]string{name}, args...),
		Stderr: tt.stderr,
	}
	stdout, err := tt.cmd.StdoutPipe()
	if err != nil {
		tt.Fatal(err)
	}
	tt.stdout = bufio.NewReader(stdout)
	if tt.stdin, err = tt.cmd.StdinPipe(); err != nil {
		tt.Fatal(err)
	}
	if err := tt.cmd.Start(); err != nil {
		tt.Fatal(err)
	}
}

// InputLine writes the given text to the child's stdin.
// This method can also be called from an expect template, e.g.:
//
//     gendchain.expect(`Passphrase: {{.InputLine "password"}}`)
func (tt *TestCmd) InputLine(s string) string {
	io.WriteString(tt.stdin, s+"\n")
	return ""
}

func (tt *TestCmd) SetTemplateFunc(name string, fn interface{}) {
	if tt.Func == nil {
		tt.Func = make(map[string]interface{})
	}
	tt.Func[name] = fn
}

// Expect runs its argument as a template, then expects the
// child process to output the result of the template within 5s.
//
// If the template starts with a newline, the newline is removed
// before matching.
func (tt *TestCmd) Expect(tplsource string) {
	// Generate the expected output by running the template.
	tpl := template.Must(template.New("").Funcs(tt.Func).Parse(tplsource))
	wantbuf := new(bytes.Buffer)
	if err := tpl.Execute(wantbuf, tt.Data); err != nil {
		panic(err)
	}
	// Trim exactly one newline at the beginning. This makes tests look
	// much nicer because all expect strings are at column 0.
	want := bytes.TrimPrefix(wantbuf.Bytes(), []byte("\n"))
	if err := tt.matchExactOutput(want); err != nil {
		tt.Fatal(err)
	}
	tt.Logf("Matched stdout text:\n%s", want)
}

func (tt *TestCmd) matchExactOutput(want []byte) error {
	buf := make([]byte, len(want))
	n := 0
	tt.withKillTimeout(func() { n, _ = io.ReadFull(tt.stdout, buf) })
	buf = buf[:n]
	if n < len(want) || !bytes.Equal(buf, want) {
		// Grab any additional buffered output in case of mismatch
		// because it might help with debugging.
		buf = append(buf, make([]byte, tt.stdout.Buffered())...)
		tt.stdout.Read(buf[n:])
		// Find the mismatch position.
		for i := 0; i < n; i++ {
			if want[i] != buf[i] {
				return fmt.Errorf("output mismatch at ◊:\n---------------- (stdout text)\n%s◊%s\n---------------- (expected text)\n%s",
					buf[:i], buf[i:n], want)
			}
		}
		if n < len(want) {
			return fmt.Errorf("not enough output, got until ◊:\n---------------- (stdout text)\n%s\n---------------- (expected text)\n%s◊%s",
				buf, want[:n], want[n:])
		}
	}
	return nil
}

// ExpectRegexp expects the child process to output text matching the
// given regular expression within 5s.
//
// Note that an arbitrary amount of output may be consumed by the
// regular expression. This usually means that expect cannot be used
// after ExpectRegexp.
func (tt *TestCmd) ExpectRegexp(regex string) (*regexp.Regexp, []string) {
	regex = strings.TrimPrefix(regex, "\n")
	var (
		re      = regexp.MustCompile(regex)
		rtee    = &runeTee{in: tt.stdout}
		matches []int
	)
	tt.withKillTimeout(func() { matches = re.FindReaderSubmatchIndex(rtee) })
	output := rtee.buf.Bytes()
	if matches == nil {
		tt.Fatalf("Output did not match:\n---------------- (stdout text)\n%s\n---------------- (regular expression)\n%s",
			output, regex)
		return re, nil
	}
	tt.Logf("Matched stdout text:\n%s", output)
	var submatches []string
	for i := 0; i < len(matches); i += 2 {
		submatch := string(output[matches[i]:matches[i+1]])
		submatches = append(submatches, submatch)
	}
	return re, submatches
}

// ExpectExit expects the child process to exit within 5s without
// printing any additional text on stdout.
func (tt *TestCmd) ExpectExit() {
	var output []byte
	tt.withKillTimeout(func() {
		output, _ = ioutil.ReadAll(tt.stdout)
	})
	tt.WaitExit()
	if tt.Cleanup != nil {
		tt.Cleanup()
	}
	if len(output) > 0 {
		tt.Errorf("Unmatched stdout text:\n%s", output)
	}
}

func (tt *TestCmd) WaitExit() {
	tt.Err = tt.cmd.Wait()
}

func (tt *TestCmd) Interrupt() {
	tt.Err = tt.cmd.Process.Signal(os.Interrupt)
}

// ExitStatus exposes the process' OS exit code
// It will only return a valid value after the process has finished.
func (tt *TestCmd) ExitStatus() int {
	if tt.Err != nil {
		exitErr := tt.Err.(*exec.ExitError)
		if exitErr != nil {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus()
			}
		}
	}
	return 0
}

// StderrText returns any stderr output written so far.
// The returned text holds all log lines after ExpectExit has
// returned.
func (tt *TestCmd) StderrText() string {
	tt.stderr.mu.Lock()
	defer tt.stderr.mu.Unlock()
	return tt.stderr.buf.String()
}

func (tt *TestCmd) CloseStdin() {
	tt.stdin.Close()
}

func (tt *TestCmd) Kill() {
	tt.cmd.Process.Kill()
	if tt.Cleanup != nil {
		tt.Cleanup()
	}
}

func (tt *TestCmd) withKillTimeout(fn func()) {
	timeout := time.AfterFunc(30*time.Second, func() {
		tt.Log("killing the child process (timeout)")
		tt.Kill()
	})
	defer timeout.Stop()
	fn()
}

// testlogger logs all written lines via t.Log and also
// collects them for later inspection.
type testlogger struct {
	t   *testing.T
	mu  sync.Mutex
	buf bytes.Buffer
}

func (tl *testlogger) Write(b []byte) (n int, err error) {
	lines := bytes.Split(b, []byte("\n"))
	for _, line := range lines {
		if len(line) > 0 {
			tl.t.Logf("(stderr) %s", line)
		}
	}
	tl.mu.Lock()
	tl.buf.Write(b)
	tl.mu.Unlock()
	return len(b), err
}

// runeTee collects text read through it into buf.
type runeTee struct {
	in interface {
		io.Reader
		io.ByteReader
		io.RuneReader
	}
	buf bytes.Buffer
}

func (rtee *runeTee) Read(b []byte) (n int, err error) {
	n, err = rtee.in.Read(b)
	rtee.buf.Write(b[:n])
	return n, err
}

func (rtee *runeTee) ReadRune() (r rune, size int, err error) {
	r, size, err = rtee.in.ReadRune()
	if err == nil {
		rtee.buf.WriteRune(r)
	}
	return r, size, err
}

func (rtee *runeTee) ReadByte() (b byte, err error) {
	b, err = rtee.in.ReadByte()
	if err == nil {
		rtee.buf.WriteByte(b)
	}
	return b, err
}