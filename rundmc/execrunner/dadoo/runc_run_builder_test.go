package dadoo_test

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/rundmc/execrunner/dadoo"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RuncRunBuilder", func() {
	const (
		runtimePath = "container_funtime"
		logfilePath = "a-logfile"
		processPath = "/a/path/to/a/process/dir"
		ctrHandle   = "a-handle"
	)

	It("builds a runc exec command for the non-tty case", func() {
		cmd := dadoo.BuildRuncCommand(runtimePath, "exec", processPath, ctrHandle, "", logfilePath)
		Expect(cmd.Path).To(Equal(runtimePath))
		Expect(cmd.Args).To(Equal([]string{
			runtimePath,
			"--debug", "--log", logfilePath, "--log-format", "json",
			"exec",
			"-d", "--pid-file", filepath.Join(processPath, "pidfile"),
			"-p", fmt.Sprintf("/proc/%d/fd/0", os.Getpid()),
			ctrHandle,
		}))
	})

	It("builds a runc exec command for the tty case", func() {
		cmd := dadoo.BuildRuncCommand(runtimePath, "exec", processPath, ctrHandle, "path/to/socketfile", logfilePath)
		Expect(cmd.Path).To(Equal(runtimePath))
		Expect(cmd.Args).To(Equal([]string{
			runtimePath,
			"--debug", "--log", logfilePath, "--log-format", "json",
			"exec",
			"-d", "--pid-file", filepath.Join(processPath, "pidfile"),
			"-p", fmt.Sprintf("/proc/%d/fd/0", os.Getpid()),
			"--tty", "--console-socket", "path/to/socketfile",
			ctrHandle,
		}))
	})

	It("builds a runc run command for the non-tty case", func() {
		cmd := dadoo.BuildRuncCommand(runtimePath, "run", processPath, ctrHandle, "", logfilePath)
		Expect(cmd.Path).To(Equal(runtimePath))
		Expect(cmd.Args).To(Equal([]string{
			runtimePath,
			"--debug", "--log", logfilePath, "--log-format", "json",
			"run",
			"-d", "--pid-file", filepath.Join(processPath, "pidfile"),
			"--no-new-keyring", "-b", processPath,
			ctrHandle,
		}))
	})

	It("builds a runc run command for the tty case", func() {
		cmd := dadoo.BuildRuncCommand(runtimePath, "run", processPath, ctrHandle, "/some/socket", logfilePath)
		Expect(cmd.Path).To(Equal(runtimePath))
		Expect(cmd.Args).To(Equal([]string{
			runtimePath,
			"--debug", "--log", logfilePath, "--log-format", "json",
			"run",
			"-d", "--pid-file", filepath.Join(processPath, "pidfile"),
			"--no-new-keyring", "-b", processPath,
			"--console-socket", "/some/socket",
			ctrHandle,
		}))
	})
})
