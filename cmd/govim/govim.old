// govim is a Go-based Vim8 plugin host for Go plugins, designed to make
// writing Go in Vim8 easier by making it easier to write plugins for Vim8 by
// allowing you to write Go instead of VimScript/Python etc. Well, that was a
// mouthful.
package main // import "myitcv.io/govim/cmd/govim"
import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"time"
)

//go:generate pkgconcat -out gen_cliflag.go myitcv.io/_tmpls/cliflag

const (
	flagServer = "s"
	flagAddr   = "addr"
	flagDebug  = "debug"

	serverRetryDelay    = 10 * time.Millisecond
	serverRetryAttempts = 50
)

var (
	fServer = flag.Bool(flagServer, false, "run as server daemon")
	fAddr   = flag.String(flagAddr, ":1982", "the address on which -s mode will listen")
	fDebug  = flag.Bool(flagDebug, false, "debug output")

	// TODO probably need option to log output when in server mode
)

func main() {
	setupAndParseFlags("")

	if *fServer {
		runAsServer()
	} else {
		tryStartServer()
	}
}

func tryStartServer() {
	path, err := os.Executable()
	if err != nil {
		fatalf("failed to detect self as executable: %v", err)
	}
	args := []string{path, "-" + flagServer, "-" + flagAddr, *fAddr}
	if *fDebug {
		args = append(args, "-"+flagDebug)
	}
	wd := getHome()

	stdin, err := os.Open(os.DevNull)
	if err != nil {
		fatalf("failed to open %v for stdin: %v", os.DevNull, err)
	}
	stdout, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		fatalf("failed to open %v for stdout: %v", os.DevNull, err)
	}
	stderr, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		fatalf("failed to open %v for stderr: %v", os.DevNull, err)
	}

	procattr := os.ProcAttr{Dir: wd, Env: os.Environ(), Files: []*os.File{stdin, stdout, stderr}}
	p, err := os.StartProcess(path, args, &procattr)
	if err != nil {
		fatalf("failed to start %q in %v: %v", strings.Join(args, " "), wd, err)
	}

	p.Release()

	// we we attempt to connect to the server and exit 0 when we succeed or non
	// zero after a nominal number of retries with delays between each
	for i := 0; i < serverRetryAttempts; i++ {
		conn, err := net.Dial("tcp", *fAddr)
		if err != nil {
			time.Sleep(serverRetryDelay)
			continue
		}

		// at this point we should be able to decode a [chanid, ]

		var m chanMsg
		dec := json.NewDecoder(conn)

		if err := dec.Decode(&m); err != nil {
			fatalf("unexpected error: %v", err)
		}

		conn.Close()
		infof("connected after %v attempts\n", i+1)
		return
	}

	fatalf("failed to connect to server")
}

// TODO this belongs somewhere else...
func getHome() string {
	var envVar string
	switch runtime.GOOS {
	case "windows":
		fatalf("don't yet support windows")
	case "plan9":
		envVar = "home"
	default:
		envVar = "HOME"
	}
	dir, ok := os.LookupEnv(envVar)
	if !ok {
		fatalf("failed to resolve env var %v", envVar)
	}
	return dir
}

func debugf(w io.Writer, format string, args ...interface{}) {
	if *fDebug {
		fmt.Fprintf(w, format+"\n", args...)
	}
}
