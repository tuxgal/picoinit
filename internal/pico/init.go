package pico

import (
	"fmt"
	"os"

	"github.com/tuxgal/tuxlogi"
)

// initImpl is the implementation of picoinit, the combo init and service
// manager.
type initImpl struct {
	// Logger used by init.
	log tuxlogi.Logger
	// State machine.
	state *stateMachine
	// Signal manager.
	signals *signalManager
	// Service/Hook janitor.
	janitor *serviceJanitor
}

// NewInit instantiates a Pico Init/Service Manager combo, performs the
// necessary initialization for the init responsibilities, launches the
// pre-hook (if one was specified) and launches the specified list of
// services. The call will block till the pre-hook exits prior to
// launching the services. The call does not block on all services to
// exit. Instead use Init.Wait() to block on the termination of all the
// services.
func NewInit(config *InitConfig) (Init, error) {
	init := &initImpl{
		log:     config.Log,
		state:   newStateMachine(config.Log),
		signals: newSignalManager(config.Log),
	}
	init.state.set(stateInitializing)

	err := init.checkPid1()
	if err != nil {
		return nil, err
	}

	err = init.launchPreHook(config.PreHook)
	if err != nil {
		return nil, err
	}

	err = init.launchServices(config.Services...)
	if err != nil {
		return nil, err
	}

	return init, nil
}

// Wait performs a blocking wait for all the launched services to terminate.
// Once the first launched service terminates, init initiates the shut down
// sequence terminating all remaining services and returns the exit status
// based on single service mode or multi service mode.
// In single service mode, the exit status is the same as that of the
// service which exited. In multi service mode, the exit status is the
// same as the first service which exited if non-zero, 77 otherwise.
func (i *initImpl) Wait() int {
	if i.janitor == nil {
		i.shutDown()
		return 77
	}

	serv, exitStatus := i.janitor.wait()
	i.log.Infof("Shutting down since service: %v terminated", serv)

	i.shutDown()
	return exitStatus
}

// launchPreHook launches the specified pre-hook and waits till it
// terminates.
func (i *initImpl) launchPreHook(hook *Hook) error {
	if hook == nil {
		return nil
	}

	i.state.set(stateLaunchingPreHook)

	// Build a new repo and janitor just for managing the pre-hook.
	janitor, repo := buildJanitor(i.log, i.signals, false)
	i.janitor = janitor

	err := launchHook(i.log, repo, hook)
	if err != nil {
		i.shutDown()
		return fmt.Errorf("failed to launch pre-hook, reason: %v", err)
	}

	i.janitor.wait()
	shutDownJanitor(i.signals, i.janitor)
	i.janitor = nil
	return nil
}

// launchServices launches the specified list of services.
func (i *initImpl) launchServices(services ...*Service) error {
	i.state.set(stateLaunchingServices)

	if len(services) == 0 {
		i.log.Warnf("Empty list of services specified")
		return nil
	}

	// Build the repo and janitor that will be used for managing
	// the services to be launched.
	multiServiceMode := len(services) > 1
	janitor, repo := buildJanitor(i.log, i.signals, multiServiceMode)
	i.janitor = janitor

	err := launchServices(i.log, repo, services...)
	if err != nil {
		i.shutDown()
		return fmt.Errorf("failed to launch services, reason: %v", err)
	}

	i.state.set(stateRunning)
	return nil
}

// shutDown terminates any running services launched by Init, unregisters
// notifications for all signals, and frees up any other monitoring resources.
func (i *initImpl) shutDown() {
	i.state.set(stateTerminatingEntities)
	if i.janitor != nil {
		shutDownJanitor(i.signals, i.janitor)
	}

	i.state.set(stateShuttingDown)
	i.signals.shutDown()

	i.state.set(stateHalted)
	i.log.Infof("All services have terminated!")
}

func (i *initImpl) checkPid1() error {
	pid := os.Getpid()
	if pid != 1 {
		return fmt.Errorf("only running as pid 1 is supported, current pid: %d. Instead please launch picoinit as the container's entrypoint", pid)
	}
	i.log.Infof("Running as pid: %d", pid)
	return nil
}

// buildJanitor builds a janitor and a repository, and associates them with
// the specified signal manager.
func buildJanitor(log tuxlogi.Logger, signals *signalManager, multiServiceMode bool) (*serviceJanitor, *serviceRepo) {
	signals.clearReapObserver()
	signals.clearRepo()
	repo := newServiceRepo(log)
	janitor := newServiceJanitor(log, repo, multiServiceMode)
	signals.setRepo(repo)
	signals.setReapObserver(janitor.handleProcTerminaton)
	return janitor, repo
}

// shutDownJanitor shuts down and cleans up the specified janitor.
func shutDownJanitor(signals *signalManager, janitor *serviceJanitor) {
	signals.clearReapObserver()
	janitor.shutDown(signals)
	signals.clearRepo()
}
