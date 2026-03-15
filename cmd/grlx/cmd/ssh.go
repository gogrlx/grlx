package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/nats-io/nats.go"

	"github.com/gogrlx/grlx/v2/internal/api/client"
	"github.com/gogrlx/grlx/v2/internal/shell"
	"github.com/gogrlx/grlx/v2/internal/sshpicker"
)

var (
	sshShell  string
	sshCohort string
)

var sshCmd = &cobra.Command{
	Use:   "ssh [sprout]",
	Short: "Open an interactive shell on a sprout over NATS",
	Long: `Open a remote interactive shell session on a sprout.

The connection is relayed through NATS — no direct SSH or network
access to the sprout is required. The farmer validates the request
and the sprout spawns a PTY-backed shell process.

Use -C/--cohort to target a cohort. If the cohort resolves to
multiple sprouts, an interactive picker is shown.

Press Ctrl-D or type 'exit' to end the session.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSSH,
}

func init() {
	sshCmd.Flags().StringVar(&sshShell, "shell", "", "shell to use on the sprout (default: /bin/sh)")
	sshCmd.Flags().StringVarP(&sshCohort, "cohort", "C", "", "cohort name — resolve to sprouts and pick one")
	rootCmd.AddCommand(sshCmd)
}

func runSSH(cmd *cobra.Command, args []string) error {
	sproutID, err := resolveSSHTarget(args)
	if err != nil {
		return err
	}

	return connectSSH(sproutID)
}

// resolveSSHTarget determines which sprout to connect to based on args and flags.
func resolveSSHTarget(args []string) (string, error) {
	hasDirect := len(args) == 1
	hasCohort := sshCohort != ""

	if hasDirect && hasCohort {
		return "", fmt.Errorf("cannot specify both a sprout argument and --cohort (-C)")
	}
	if !hasDirect && !hasCohort {
		return "", fmt.Errorf("specify a sprout name or use --cohort (-C)")
	}

	if hasDirect {
		return args[0], nil
	}

	// Resolve cohort to sprout list.
	sprouts, err := client.ResolveCohort(sshCohort)
	if err != nil {
		return "", fmt.Errorf("cohort %q: %w", sshCohort, err)
	}

	if len(sprouts) == 1 {
		fmt.Fprintf(os.Stderr, "Cohort %q → %s\n", sshCohort, sprouts[0])
		return sprouts[0], nil
	}

	// Multiple sprouts — interactive picker.
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("cohort %q has %d sprouts — interactive picker requires a terminal", sshCohort, len(sprouts))
	}

	return sshpicker.Run(sshCohort, sprouts)
}

// connectSSH opens the NATS shell session to the given sprout.
func connectSSH(sproutID string) error {
	// Get terminal dimensions.
	cols, rows := 80, 24
	if w, h, err := term.GetSize(int(os.Stdin.Fd())); err == nil {
		cols, rows = w, h
	}

	// Request a shell session from the farmer.
	req := shell.CLIStartRequest{
		SproutID: sproutID,
		Cols:     cols,
		Rows:     rows,
		Shell:    sshShell,
	}

	resp, err := client.NatsRequest("shell.start", req)
	if err != nil {
		return fmt.Errorf("shell.start: %w", err)
	}

	var session shell.StartResponse
	if err := json.Unmarshal(resp, &session); err != nil {
		return fmt.Errorf("invalid session response: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Connected to %s (session %s)\n", sproutID, session.SessionID)

	// Put terminal in raw mode.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw mode: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	nc := client.NatsConn
	done := make(chan struct{})

	// Subscribe to output from sprout → write to stdout.
	outputSub, err := nc.Subscribe(session.OutputSubject, func(msg *nats.Msg) {
		os.Stdout.Write(msg.Data)
	})
	if err != nil {
		return fmt.Errorf("subscribe output: %w", err)
	}
	defer outputSub.Unsubscribe()

	// Subscribe to done signal.
	doneSub, err := nc.Subscribe(session.DoneSubject, func(msg *nats.Msg) {
		close(done)
	})
	if err != nil {
		return fmt.Errorf("subscribe done: %w", err)
	}
	defer doneSub.Unsubscribe()

	// Handle terminal resize (SIGWINCH).
	sigWinch := make(chan os.Signal, 1)
	signal.Notify(sigWinch, syscall.SIGWINCH)
	go func() {
		for {
			select {
			case <-sigWinch:
				if w, h, err := term.GetSize(int(os.Stdin.Fd())); err == nil {
					resize := shell.ResizeMessage{Cols: w, Rows: h}
					data, _ := json.Marshal(resize)
					nc.Publish(session.ResizeSubject, data)
				}
			case <-done:
				return
			}
		}
	}()

	// Read stdin → publish to sprout input subject.
	go func() {
		buf := make([]byte, 1024)
		for {
			n, readErr := os.Stdin.Read(buf)
			if n > 0 {
				nc.Publish(session.InputSubject, buf[:n])
			}
			if readErr != nil {
				if readErr != io.EOF {
					// Terminal closed or error.
				}
				return
			}
		}
	}()

	// Wait for the shell to exit.
	<-done
	fmt.Fprintf(os.Stderr, "\r\nSession ended.\n")
	return nil
}
