package main

import (
	"fmt"
	"log"
	"os"

	"github.com/opentable/sous/cli"
	"github.com/opentable/sous/graph"
	"github.com/opentable/sous/util/logging"
)

// Sous is the Sous CLI root command.
var Sous = &cli.Sous{
	Version:   Version,
	OS:        OS,
	Arch:      Arch,
	GoVersion: GoVersion,
}

func main() {
	defer handlePanic()
	os.Exit(action())
}

func action() int {
	log.SetFlags(log.Flags() | log.Lshortfile)

	di := graph.BuildGraph(Sous.Version, os.Stdin, os.Stdout, os.Stderr)
	type logSetScoop struct {
		*logging.LogSet
	}
	lss := &logSetScoop{}
	if err := di.Inject(lss); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 70
	}
	defer func() {
		lss.LogSet.AtExit()
	}()

	c, err := cli.NewSousCLI(di, Sous, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 70
	}
	c.LogSink = lss.LogSet

	return c.Invoke(os.Args).ExitCode()
}

// handlePanic gives us one last chance to send a message to the user in case a
// panic leaks right up to the top of the program. You can disable this message
// for brevity of output by setting DEBUG=YES
func handlePanic() {
	if os.Getenv("DEBUG") == "YES" {
		return
	}

	fmt.Println(panicMessage)
	fmt.Printf("Sous Version: %s\n\n", Version)
}

const panicMessage = `
################################################################################
#                                                                              #
#                                       OOPS                                   #
#                                                                              #
#        Sous has panicked, due to programmer error. Please report this        #
#        to the project maintainers at:                                        #
#                                                                              #
#                https://github.com/opentable/sous/issues                      #
#                                                                              #
#        Please include this entire message and the stack trace below          #
#        and we will investigate and fix it as soon as possible.               #
#                                                                              #
#        Thanks for your help in improving Sous for all!                       #
#                                                                              #
#        - The OpenTable DevTools Team                                         #
#                                                                              #
################################################################################
`
