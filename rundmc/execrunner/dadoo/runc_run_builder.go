package dadoo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func BuildRuncCommands(runtimePath, runMode, processPath, containerHandle, ttyConsoleSocket, logfilePath string) []*exec.Cmd {
	globalArgs := []string{"--debug", "--log", logfilePath, "--log-format", "json"}
	runtimeArgs := append(globalArgs, runMode, "--pid-file", filepath.Join(processPath, "pidfile"))
	runtimeArgs = append(runtimeArgs, runmodeArgs(runMode, processPath)...)
	runtimeArgs = append(runtimeArgs, ttyArgs(runMode, ttyConsoleSocket)...)
	runtimeArgs = append(runtimeArgs, containerHandle)

	cmds := []*exec.Cmd{exec.Command(runtimePath, runtimeArgs...)}
	if runMode == "create" {
		args := append(globalArgs, "start", containerHandle)
		cmds = append(cmds, exec.Command(runtimePath, args...))
	}
	return cmds
}

func runmodeArgs(runMode, bundlePath string) []string {
	if runMode == "create" {
		return []string{"--no-new-keyring", "--bundle", bundlePath}
	}

	return []string{"--detach", "--process", fmt.Sprintf("/proc/%d/fd/0", os.Getpid())}
}

func ttyArgs(runMode, ttyConsoleSocket string) []string {
	args := []string{}
	if ttyConsoleSocket == "" {
		return args
	}

	if runMode == "exec" {
		args = append(args, "--tty")
	}

	return append(args, "--console-socket", ttyConsoleSocket)
}
