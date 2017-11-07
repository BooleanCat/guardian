package peas

import (
	"os/exec"
	"syscall"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/logging"
)

type pearocess struct {
	id              string
	doneCh          <-chan error
	volumeDestroyer func()
}

func (p pearocess) ID() string { return p.id }

func (p pearocess) Wait() (int, error) {
	runcRunErr := <-p.doneCh
	defer p.volumeDestroyer()
	if runcRunErr == nil {
		return 0, nil
	}
	if wrappedErr, ok := runcRunErr.(logging.WrappedError); ok {
		if exitErr, ok := wrappedErr.Underlying.(*exec.ExitError); ok {
			return exitErr.Sys().(syscall.WaitStatus).ExitStatus(), nil
		}
	}

	return -1, runcRunErr
}

func (p pearocess) SetTTY(garden.TTYSpec) error { return nil }
func (p pearocess) Signal(garden.Signal) error  { return nil }
