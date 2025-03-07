package pico

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/tuxgal/tuxlogi"
	"golang.org/x/sys/unix"
)

// serviceLauncher allows launching services.
type serviceLauncher struct {
	// Logger used by the service launcher.
	log tuxlogi.Logger
	// Service repository.
	repo launcherRepo
}

// launcherRepo is the repository interface used by the launcher to add
// the launched services to the repository.
type launcherRepo interface {
	addService(serv *launchedServiceOrHook)
}

// launchHook launches the specified hook and waits till it terminates
// prior to exiting.
func launchHook(log tuxlogi.Logger, repo launcherRepo, hook *Hook) error {
	sl := &serviceLauncher{
		log:  log,
		repo: repo,
	}
	return sl.startService(false, hook.Cmd, hook.Args...)
}

// launchServices launches the specified list of services and updates the
// service list in the specified repository.
func launchServices(log tuxlogi.Logger, repo launcherRepo, services ...*Service) error {
	sl := &serviceLauncher{
		log:  log,
		repo: repo,
	}
	return sl.launchServices(services...)
}

// launchServices launches the specified list of services and updates the
// service list in the specified repository.
func (s *serviceLauncher) launchServices(services ...*Service) error {
	multiServiceMode := len(services) > 1
	for _, serv := range services {
		err := s.startService(multiServiceMode, serv.Cmd, serv.Args...)
		if err != nil {
			return err
		}
	}
	return nil
}

// startService launches the specified service binary invoking it with the
// specified list of command line arguments.
func (s *serviceLauncher) startService(multiServiceMode bool, bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.SysProcAttr = &unix.SysProcAttr{
		// Use a new process group for the child.
		Setpgid: true,
	}
	if !multiServiceMode {
		// Only in single service mode we redirect stdin to the
		// one and only service that is being launched.
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to launch service %q %q, reason: %v", bin, args, err)
	}

	if !multiServiceMode {
		// Only in single service mode, attach the stdin TTY (if present) to
		// the one and only service launched.
		err := s.attachStdinTTY(cmd.Process.Pid)
		if err != nil {
			return err
		}
	}

	proc := &launchedServiceOrHook{}
	proc.pid = cmd.Process.Pid
	proc.entity.cmd = bin
	proc.entity.args = make([]string, len(args))
	copy(proc.entity.args, args)
	s.repo.addService(proc)

	s.log.Infof("Launched service %q pid: %d", bin, proc.pid)
	return nil
}

// attachStdinTTY attaches the TTY to the process with the specified pid.
func (s *serviceLauncher) attachStdinTTY(pid int) error {
	err := unix.IoctlSetPointerInt(unix.Stdin, unix.TIOCSPGRP, pid)
	if err == nil {
		s.log.Debugf("Attached TTY of stdin to pid: %d", pid)
		return nil
	}

	var errNo unix.Errno
	if errors.As(err, &errNo) {
		if errNo == unix.ENOTTY {
			s.log.Debugf("No stdin TTY found to attach, ignoring")
			return nil
		}
		return fmt.Errorf("tcsetpgrp failed attempting to attach stdin TTY, errno: %v", errNo)
	}
	return fmt.Errorf("tcsetpgrp failed attempting to attach stdin TTY, reason: %T %v", err, err)
}
