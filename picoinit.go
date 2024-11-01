// Command picoinit is a combo minimalistic init and service manager process
// to launch and manage multiple services within a single docker container.
package main

import (
	"os"

	"github.com/tuxgal/picoinit/internal/pico"
	"github.com/tuxgal/tuxlog"
	"github.com/tuxgal/tuxlogi"
)

var (
	log = buildLogger()
)

func buildLogger() tuxlogi.Logger {
	config := tuxlog.NewConsoleLoggerConfig()
	config.MaxLevel = tuxlog.LvlWarn
	return tuxlog.NewLogger(config)
}

func run() int {
	inv, err := parseFlags()
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Invocation: %v", inv)

	init, err := pico.NewInit(&pico.InitConfig{
		Log:      log,
		PreHook:  inv.preHook,
		Services: inv.services,
	})
	if err != nil {
		log.Errorf("picoinit failed to initialize and launch the services, reason: %v", err)
		return 1
	}
	exitCode := init.Wait()

	log.Infof("picoinit exiting with status code: %d", exitCode)
	return exitCode
}

func main() {
	os.Exit(run())
}
