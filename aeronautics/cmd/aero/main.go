// Command aero runs the calculator's HTTP boundary and prints its contract.
//
// It is the deployment unit: a static host does not execute the calculation
// core, so a worksheet needs this process running alongside its assets.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"yalb.aero/api"
	"yalb.aero/calculator"
	"yalb.aero/httpapi"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "aero: "+err.Error())
		os.Exit(1)
	}
}

// run dispatches the subcommand. It takes its arguments and its output so the
// commands are testable without a process.
func run(args []string, out *os.File) error {
	command := "version"
	if len(args) > 0 {
		command = args[0]
		args = args[1:]
	}
	switch command {
	case "version":
		_, err := fmt.Fprintln(out, "yalb.aero "+calculator.Version+" contract "+api.ContractVersion)
		return err //nolint:wrapcheck // the writer's own error, with nothing to add
	case "contract":
		_, err := fmt.Fprint(out, api.TypeScript())
		return err //nolint:wrapcheck // the writer's own error, with nothing to add
	case "serve":
		return serve(args, out)
	default:
		return errors.New("unknown command " + command + "; use version, contract or serve")
	}
}

// serve runs the HTTP boundary until the process is interrupted.
func serve(args []string, out *os.File) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(out)
	addr := flags.String("addr", "127.0.0.1:8081", "address to listen on")
	if err := flags.Parse(args); err != nil {
		return err //nolint:wrapcheck // flag's own message is the whole story
	}

	server := &http.Server{
		Addr:              *addr,
		Handler:           httpapi.Handler(api.NewService()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	failed := make(chan error, 1)
	go func() {
		if _, err := fmt.Fprintln(out, "aero serving "+httpapi.Prefix+" on http://"+*addr); err != nil {
			failed <- err
			return
		}
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			failed <- err
		}
	}()

	select {
	case err := <-failed:
		return err
	case <-ctx.Done():
	}

	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		return errors.New("shutting down: " + err.Error())
	}
	return nil
}
