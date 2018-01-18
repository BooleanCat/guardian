package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/execrunner/dadoo"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/opencontainers/runc/libcontainer/system"

	cmsg "github.com/opencontainers/runc/libcontainer/utils"
)

const (
	MaxSocketDirPathLength = 80
	RuncExecTimeout        = time.Second * 5
)

func main() {
	os.Exit(run())
}

func run() int {
	tty := flag.Bool("tty", false, "tty requested")
	socketDirPath := flag.String("socket-dir-path", "", "path to a dir in which to store console sockets")
	flag.Parse()

	runMode := flag.Args()[0] // exec or create
	runtime := flag.Args()[1] // e.g. runc
	processStateDir := flag.Args()[2]
	containerId := flag.Args()[3]

	signals := make(chan os.Signal, 100)
	signal.Notify(signals, syscall.SIGCHLD)

	runcExitCodePipe := os.NewFile(3, "/proc/self/fd/3")
	logFile := fmt.Sprintf("/proc/%d/fd/4", os.Getpid())
	logFD := os.NewFile(4, "/proc/self/fd/4")
	syncPipe := os.NewFile(5, "/proc/self/fd/5")
	pidFilePath := filepath.Join(processStateDir, "pidfile")

	stdinR, stdoutW, stderrW, err := openStdioAndExitFifos(processStateDir)
	defer closeFile(stdinR, stdoutW, stderrW)
	if err != nil {
		fmt.Println(err)
		return 2
	}

	syncPipe.Write([]byte{0})

	stdoutR, stderrR, err := openStdioKeepAlivePipes(processStateDir)
	defer closeFile(stdoutR, stderrR)
	if err != nil {
		fmt.Println(err)
		return 2
	}

	ioWg := &sync.WaitGroup{}
	var runtimeCmds []*exec.Cmd
	if *tty {
		winsz, err := openFile(filepath.Join(processStateDir, "winsz"), os.O_RDWR)
		defer closeFile(winsz)
		if err != nil {
			fmt.Println(err)
			return 2
		}

		if len(*socketDirPath) > MaxSocketDirPathLength {
			return logAndExit(fmt.Sprintf("value for --socket-dir-path cannot exceed %d characters in length", MaxSocketDirPathLength))
		}
		ttySocketPath := setupTTYSocket(stdinR, stdoutW, winsz, pidFilePath, *socketDirPath, ioWg)
		runtimeCmds = dadoo.BuildRuncCommands(runtime, runMode, processStateDir, containerId, ttySocketPath, logFile)
	} else {
		runtimeCmds = dadoo.BuildRuncCommands(runtime, runMode, processStateDir, containerId, "", logFile)
		createCtrCmd := runtimeCmds[0]
		createCtrCmd.Stdin = stdinR
		createCtrCmd.Stdout = stdoutW
		createCtrCmd.Stderr = stderrW
	}

	// we need to be the subreaper so we can wait on the detached container process
	system.SetSubreaper(os.Getpid())

	// TODO holy leaky abstractions batman
	createStartSplit := false

	// Run any runc setup commands. In the exec case, there will be none, and
	// this loop will not execute. In the run case, there will be one: `runc
	// create` the container, so that we can start it later.
	for i := 0; i < len(runtimeCmds)-1; i++ {
		createStartSplit = true
		if err := runtimeCmds[i].Run(); err != nil {
			return logAndExit(fmt.Sprintf(
				"setup command '%s' failed: %s", strings.Join(runtimeCmds[i].Args, " "), err,
			))
		}
	}

	startCtrCmd := runtimeCmds[len(runtimeCmds)-1]
	// TODO: this is not currently driven by a test!
	startCtrCmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}
	if err := startCtrCmd.Start(); err != nil {
		runcExitCodePipe.Write([]byte{2})
		return 2
	}

	procs := make(chan proc, 100) // TODO buffer size
	runcExitStatus := make(chan int)
	go reap(signals, procs, startCtrCmd.Process.Pid, runcExitStatus)
	runcExitCodeWritten := make(chan struct{})
	go awaitRuncExit(runcExitStatus, logFD, runcExitCodePipe, runcExitCodeWritten)

	if !createStartSplit {
		<-runcExitCodeWritten
	}

	userProcessPid, err := parsePid(pidFilePath)
	check(err, 44)

	userProcessExitStatus := awaitUserProcessExit(processStateDir, userProcessPid, procs, ioWg)
	select {
	case <-runcExitCodeWritten:
	case <-time.After(time.Second * 3): // TODO arbitrary?
	}
	return userProcessExitStatus
}

// If gdn server process dies, we need dadoo to keep stdout/err reader
// FDs so that Linux does not SIGPIPE the user process if it tries to use its end of
// these pipes.
func openStdioKeepAlivePipes(processStateDir string) (io.ReadCloser, io.ReadCloser, error) {
	keepStdoutAlive, err := openFile(filepath.Join(processStateDir, "stdout"), os.O_RDONLY)
	if err != nil {
		return nil, nil, err
	}
	keepStderrAlive, err := openFile(filepath.Join(processStateDir, "stderr"), os.O_RDONLY)
	if err != nil {
		return nil, nil, err
	}
	return keepStdoutAlive, keepStderrAlive, nil
}

type proc struct {
	pid    int
	status syscall.WaitStatus
}

func reap(signals <-chan os.Signal, procs chan<- proc, runcPid int, runcExit chan<- int) {
	for range signals {
		for {
			var status syscall.WaitStatus
			wpid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, &syscall.Rusage{})
			if err != nil || wpid <= 0 {
				break // wait for next SIGCHLD
			}
			if wpid == runcPid {
				runcExit <- status.ExitStatus()
			}
			procs <- proc{pid: wpid, status: status}
		}
	}

	os.Exit(logAndExit("ran out of signals")) // cant happen
}

func awaitRuncExit(runcExitStatusCh <-chan int, logFD, runcExitCodePipe *os.File, runcExitCodeWritten chan<- struct{}) {
	runcExitStatus := <-runcExitStatusCh
	logFD.Close() // No more logs from runc so close fd

	// also check that masterFD is received and streaming or whatevs
	runcExitCodePipe.Write([]byte{byte(runcExitStatus)})
	if runcExitStatus != 0 {
		fmt.Printf("runc exited with %d\n", runcExitStatus)
		os.Exit(3) // nothing to wait for, container didn't launch
	}
	close(runcExitCodeWritten)
}

func awaitUserProcessExit(processStateDir string, userProcessPid int, procs <-chan proc, ioWg *sync.WaitGroup) int {
	for proc := range procs {
		if proc.pid == userProcessPid {
			exitCode := proc.status.ExitStatus()
			if proc.status.Signaled() {
				exitCode = 128 + int(proc.status.Signal())
			}

			ioWg.Wait() // wait for full output to be collected

			check(ioutil.WriteFile(filepath.Join(processStateDir, "exitcode"), []byte(strconv.Itoa(exitCode)), 0600), 3)
			return exitCode
		}
	}

	return logAndExit("ran out of child processes") // can't happen
}

func openStdioAndExitFifos(processStateDir string) (io.ReadCloser, io.WriteCloser, io.WriteCloser, error) {
	stdin, err := openFile(filepath.Join(processStateDir, "stdin"), os.O_RDONLY)
	if err != nil {
		return nil, nil, nil, err
	}
	stdout, err := openFile(filepath.Join(processStateDir, "stdout"), os.O_WRONLY)
	if err != nil {
		return nil, nil, nil, err
	}
	stderr, err := openFile(filepath.Join(processStateDir, "stderr"), os.O_WRONLY)
	if err != nil {
		return nil, nil, nil, err
	}
	// open just so guardian can detect it being closed when we exit
	if _, err := openFile(filepath.Join(processStateDir, "exit"), os.O_RDWR); err != nil {
		return nil, nil, nil, err
	}
	return stdin, stdout, stderr, nil
}

func openFile(path string, flags int) (*os.File, error) {
	return os.OpenFile(path, flags, 0600)
}

func setupTTYSocket(stdin io.Reader, stdout io.Writer, winszFifo io.Reader, pidFilePath, sockDirBase string, ioWg *sync.WaitGroup) string {
	sockDir, err := ioutil.TempDir(sockDirBase, "")
	check(err, 4)

	ttySockPath := filepath.Join(sockDir, "tty.sock")
	l, err := net.Listen("unix", ttySockPath)
	check(err, 5)

	//go to the background and set master
	go func(ln net.Listener) (err error) {
		// if any of the following errors, it means runc has connected to the
		// socket, so it must've started, thus we might need to kill the process
		defer func() {
			if err != nil {
				killProcess(pidFilePath)
				check(err, 6)
			}
		}()

		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Close ln, to allow for other instances to take over.
		ln.Close()

		// Get the fd of the connection.
		unixconn, ok := conn.(*net.UnixConn)
		if !ok {
			return
		}

		socket, err := unixconn.File()
		if err != nil {
			return
		}
		defer socket.Close()

		// Get the master file descriptor from runC.
		master, err := cmsg.RecvFd(socket)
		if err != nil {
			return
		}

		if err = os.RemoveAll(sockDir); err != nil {
			return
		}

		if err = setOnlcr(master); err != nil {
			return
		}
		streamProcess(master, stdin, stdout, winszFifo, ioWg)

		return
	}(l)

	return ttySockPath
}

func streamProcess(m *os.File, stdin io.Reader, stdout io.Writer, winszFifo io.Reader, ioWg *sync.WaitGroup) {
	ioWg.Add(1)
	go func() {
		defer ioWg.Done()
		io.Copy(stdout, m)
	}()

	go io.Copy(m, stdin)

	go func() {
		for {
			var winSize garden.WindowSize
			if err := json.NewDecoder(winszFifo).Decode(&winSize); err != nil {
				fmt.Printf("invalid winsz event: %s\n", err)
				continue // not much we can do here..
			}
			dadoo.SetWinSize(m, winSize)
		}
	}()
}

func killProcess(pidFilePath string) {
	pid, err := readPid(pidFilePath)
	if err == nil {
		syscall.Kill(pid, syscall.SIGKILL)
	}
}

func readPid(pidFilePath string) (int, error) {
	retrier := retrier.New(retrier.ConstantBackoff(20, 500*time.Millisecond), nil)
	var (
		pid = -1
		err error
	)
	retrier.Run(func() error {
		pid, err = parsePid(pidFilePath)
		return err
	})

	return pid, err
}

func parsePid(pidFile string) (int, error) {
	b, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return -1, err
	}

	var pid int
	if _, err := fmt.Sscanf(string(b), "%d", &pid); err != nil {
		return -1, err
	}

	return pid, nil
}

func logAndExit(msg string) int {
	fmt.Println(msg)
	return 2
}

// TODO roll back number thing
func check(err error, count int) {
	if err != nil {
		fmt.Println("!!!")
		fmt.Println(count)
		fmt.Println("!!!")
		fmt.Println(err)
		os.Exit(2)
	}
}

func closeFile(closers ...io.Closer) {
	for _, closer := range closers {
		closer.Close()
	}
}

// setOnlcr copied from runc
// https://github.com/cloudfoundry-incubator/runc/blob/02ec89829b24dfce45bb207d2344e0e6d078a93c/libcontainer/console_linux.go#L144-L160
func setOnlcr(terminal *os.File) error {
	var termios syscall.Termios

	if err := ioctl(terminal.Fd(), syscall.TCGETS, uintptr(unsafe.Pointer(&termios))); err != nil {
		return fmt.Errorf("ioctl(tty, tcgets): %s", err.Error())
	}

	termios.Oflag |= syscall.ONLCR

	if err := ioctl(terminal.Fd(), syscall.TCSETS, uintptr(unsafe.Pointer(&termios))); err != nil {
		return fmt.Errorf("ioctl(tty, tcsets): %s", err.Error())
	}

	return nil
}

func ioctl(fd uintptr, flag, data uintptr) error {
	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, flag, data); err != 0 {
		return err
	}
	return nil
}
