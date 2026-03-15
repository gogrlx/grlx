package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/gogrlx/grlx/v2/internal/api/client"
	"github.com/gogrlx/grlx/v2/internal/shell"
)

var sshShell string

var sshCmd = &cobra.Command{
	Use:   "ssh <sprout>",
	Short: "Open an interactive shell on a sprout over NATS",
	Long: `Open a remote interactive shell session on a sprout.

The connection is relayed through NATS — no direct SSH or network
access to the sprout is required. The farmer validates the request
and the sprout spawns a PTY-backed shell process.

Press Ctrl-D or type 'exit' to end the session.`,
	Args: cobra.ExactArgs(1),
	Run:  runSSH,
}

func init() {
	sshCmd.Flags().StringVar(&sshShell, "shell", "", "shell to use on the sprout (default: /bin/sh)")
	rootCmd.AddCommand(sshCmd)
}

func runSSH(cmd *cobra.Command, args []string) {
	sproutID := args[0]

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
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var session shell.StartResponse
	if err := json.Unmarshal(resp, &session); err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid session response: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Connected to %s (session %s)\n", sproutID, session.SessionID)

	// Put terminal in raw mode.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to set raw mode: %v\n", err)
		os.Exit(1)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	nc := client.NatsConn
	done := make(chan struct{})

	// Subscribe to output from sprout → write to stdout.
	outputSub, err := nc.Subscribe(session.OutputSubject, func(msg *nats.Msg) {
		os.Stdout.Write(msg.Data)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	defer outputSub.Unsubscribe()

	// Subscribe to done signal.
	doneSub, err := nc.Subscribe(session.DoneSubject, func(msg *nats.Msg) {
		close(done)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
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
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				nc.Publish(session.InputSubject, buf[:n])
			}
			if err != nil {
				if err != io.EOF {
					// Terminal closed or error.
				}
				return
			}
		}
	}()

	// Wait for the shell to exit.
	<-done
	fmt.Fprintf(os.Stderr, "\r\nSession ended.\n")
}
