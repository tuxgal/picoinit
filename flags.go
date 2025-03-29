package main

import (
	"fmt"
	"os"

	"github.com/tuxgal/picoinit/internal/pico"
)

const (
	cmdFlag     = "--picoinit-cmd"
	preHookFlag = "--picoinit-pre-hook"
)

var (
	picoInitFlags = map[string]bool{
		cmdFlag:     true,
		preHookFlag: true,
	}
)

// invocation represents the invocation of picoinit command.
type invocation struct {
	// preHook represents the hook to be executed prior to launching the
	// services.
	preHook *pico.Hook
	// services contains the list of services to be launched and managed
	// by picoinit.
	services []*pico.Service
}

// String returns the string representation of the invocation.
func (i *invocation) String() string {
	return fmt.Sprintf("{preHook: %v, services: %v}", i.preHook, i.services)
}

// argIterator allows iterating over the picoinit command line args and
// parse them into structured picoinit invocation information.
type argIterator struct {
	// args contains the list of command line args passed to the picoinit
	// command.
	args []string
	// idx represents the current index position in the iteration over
	// all the command line args.
	idx int
}

// newArgIterator instantiates a new picoinit arg iterator.
func newArgIterator(args []string, startPos int) *argIterator {
	return &argIterator{
		args: args,
		idx:  startPos,
	}
}

// captureUntilNextPicoInitFlag captures and returns all the command line
// args from the current position (exclusive) until the next command line
// arg which is a picoinit specific flag.
func (a *argIterator) captureUntilNextPicoInitFlag() ([]string, error) {
	// Move to the first arg for this flag.
	a.idx++
	if a.idx >= len(a.args) {
		return nil, nil
	}

	// Set the starting position.
	start := a.idx
	a.idx++
	for a.idx < len(a.args) && !a.picoInitFlag() {
		a.idx++
	}
	return a.args[start:a.idx], nil
}

// picoInitFlag returns true if the command line arg at the current position
// is a picoinit flag, false otherwise.
func (a *argIterator) picoInitFlag() bool {
	return picoInitFlag(a.arg())
}

// arg returns the command line arg at the current position.
func (a *argIterator) arg() string {
	return a.args[a.idx]
}

// done returns true if the iterator has traversed over all the command
// line args, false otherwise.
func (a *argIterator) done() bool {
	return a.idx == len(a.args)
}

// parseFlags parses the command line args and returns the invocation
// information for picoinit.
func parseFlags() (*invocation, error) {
	if len(os.Args) <= 1 {
		return &invocation{}, nil
	}
	if picoInitFlag(os.Args[1]) {
		return parseWithPicoInitFlags()
	}
	return parseWithoutPicoInitFlags()
}

// parseWithPicoInitFlags parses the command line args in the mode where
// picoinit specific flags are specified in the command line and are
// expected to represent every service to be launched and managed by picoinit.
func parseWithPicoInitFlags() (*invocation, error) {
	fi := newArgIterator(os.Args, 1)
	if !fi.picoInitFlag() {
		log.Fatalf("Expected a picoinit flag as the first arg, but got %q instead.", fi.arg())
	}

	var services []*pico.Service
	var preHook *pico.Hook

	for !fi.done() {
		arg := fi.arg()
		switch arg {
		case cmdFlag:
			cmdArgs, err := fi.captureUntilNextPicoInitFlag()
			if err != nil {
				return nil, err
			}
			if len(cmdArgs) == 0 {
				return nil, fmt.Errorf("specified %q without specifying any arguments", cmdFlag)
			}
			s := &pico.Service{
				Cmd:  cmdArgs[0],
				Args: cmdArgs[1:],
			}
			services = append(services, s)
		case preHookFlag:
			if preHook != nil {
				return nil, fmt.Errorf("cannot specify multiple pre-hooks")
			}
			cmdArgs, err := fi.captureUntilNextPicoInitFlag()
			if err != nil {
				return nil, err
			}
			if len(cmdArgs) == 0 {
				return nil, fmt.Errorf("specified %q without specifying any arguments", preHookFlag)
			}
			preHook = &pico.Hook{
				Cmd:  cmdArgs[0],
				Args: cmdArgs[1:],
			}
		default:
			log.Fatalf("Unexpected condition, a picoinit flag we do not handle: %q", arg)
		}
	}

	return &invocation{
		preHook:  preHook,
		services: services,
	}, nil
}

// parseWithoutPicoInitFlags parses the command line args in the mode where
// no picoinit specific flags are specified in the command line and the command
// line args represent the information for launching and managing just a
// single service.
func parseWithoutPicoInitFlags() (*invocation, error) {
	// TODO: Verify there are no picoinit flags in the command line args.
	s := &pico.Service{
		Cmd: os.Args[1],
	}
	if len(os.Args) > 2 {
		s.Args = os.Args[2:]
	}
	return &invocation{
		services: []*pico.Service{
			s,
		},
	}, nil
}

// picoInitFlag returns true if the specified command line arg is a picoinit
// specific flag, false otherwise.
func picoInitFlag(arg string) bool {
	_, ok := picoInitFlags[arg]
	return ok
}
