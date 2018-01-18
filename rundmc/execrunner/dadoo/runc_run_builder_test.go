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
		cmds := dadoo.BuildRuncCommands(runtimePath, "exec", processPath, ctrHandle, "", logfilePath)
		Expect(cmds).To(HaveLen(1))
		cmd := cmds[0]
		Expect(cmd.Path).To(Equal(runtimePath))
		Expect(cmd.Args).To(Equal([]string{
			runtimePath,
			"--debug", "--log", logfilePath, "--log-format", "json",
			"exec",
			"--pid-file", filepath.Join(processPath, "pidfile"),
			"--detach", "--process", fmt.Sprintf("/proc/%d/fd/0", os.Getpid()),
			ctrHandle,
		}))
	})

	It("builds a runc exec command for the tty case", func() {
		cmds := dadoo.BuildRuncCommands(runtimePath, "exec", processPath, ctrHandle, "path/to/socketfile", logfilePath)
		Expect(cmds).To(HaveLen(1))
		cmd := cmds[0]
		Expect(cmd.Path).To(Equal(runtimePath))
		Expect(cmd.Args).To(Equal([]string{
			runtimePath,
			"--debug", "--log", logfilePath, "--log-format", "json",
			"exec",
			"--pid-file", filepath.Join(processPath, "pidfile"),
			"--detach", "--process", fmt.Sprintf("/proc/%d/fd/0", os.Getpid()),
			"--tty", "--console-socket", "path/to/socketfile",
			ctrHandle,
		}))
	})

	It("builds runc create+start commands for the non-tty case", func() {
		cmds := dadoo.BuildRuncCommands(runtimePath, "create", processPath, ctrHandle, "", logfilePath)
		Expect(cmds).To(HaveLen(2))

		createCmd := cmds[0]
		Expect(createCmd.Path).To(Equal(runtimePath))
		Expect(createCmd.Args).To(Equal([]string{
			runtimePath,
			"--debug", "--log", logfilePath, "--log-format", "json",
			"create",
			"--pid-file", filepath.Join(processPath, "pidfile"),
			"--no-new-keyring", "--bundle", processPath,
			ctrHandle,
		}))

		runCmd := cmds[1]
		Expect(runCmd.Path).To(Equal(runtimePath))
		Expect(runCmd.Args).To(Equal([]string{
			runtimePath,
			"--debug", "--log", logfilePath, "--log-format", "json",
			"start",
			ctrHandle,
		}))
	})

	It("builds runc create+start commands for the tty case", func() {
		cmds := dadoo.BuildRuncCommands(runtimePath, "create", processPath, ctrHandle, "/some/socket", logfilePath)
		Expect(cmds).To(HaveLen(2))

		createCmd := cmds[0]
		Expect(createCmd.Path).To(Equal(runtimePath))
		Expect(createCmd.Args).To(Equal([]string{
			runtimePath,
			"--debug", "--log", logfilePath, "--log-format", "json",
			"create",
			"--pid-file", filepath.Join(processPath, "pidfile"),
			"--no-new-keyring", "--bundle", processPath,
			"--console-socket", "/some/socket",
			ctrHandle,
		}))

		runCmd := cmds[1]
		Expect(runCmd.Path).To(Equal(runtimePath))
		Expect(runCmd.Args).To(Equal([]string{
			runtimePath,
			"--debug", "--log", logfilePath, "--log-format", "json",
			"start",
			ctrHandle,
		}))
	})
})
