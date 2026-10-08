package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"project-revina/internal/config"
	"project-revina/internal/engine"
	"project-revina/internal/engine/translators"
	"project-revina/internal/hardening"
	"project-revina/profiles"
	"project-revina/ui"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

var (
	profilePath    string
	dryRun         bool
	yes            bool
	nonInteractive bool
	statePath      string
	logLevel       string
)

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devsec",
		Short: "Multi-OS environment provisioner and hardening tool",
		RunE:  run,
	}
	f := cmd.PersistentFlags()
	f.StringVar(&profilePath, "profile", "dev.yaml", "profile name (embedded) or path to a YAML profile")
	f.StringVar(&logLevel, "log-level", "info", "log level: debug, info, warn, error")
	f.StringVar(&statePath, "state", "", "audit state file (default: <config>/devsec/state.json)")

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan without changing anything")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation and apply")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "never prompt; refuse instead of assuming yes")

	cmd.AddCommand(auditCmd())

	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)
	return cmd
}

// translatorFactory is overridable in tests to avoid touching a real package
// manager or a real OS. It receives the Runner so per-OS detection (Linux/
// Windows) can probe for the available manager.
var translatorFactory = func(ctx context.Context, r engine.Runner) (engine.Translator, error) {
	switch runtime.GOOS {
	case "darwin":
		return translators.NewBrew(r)
	case "linux":
		pm, err := engine.DetectLinuxPM(ctx, r)
		if err != nil {
			return nil, err
		}
		switch pm {
		case "apt":
			return translators.NewApt(r)
		case "dnf":
			return translators.NewDnf(r)
		case "pacman":
			return translators.NewPacman(r)
		default:
			return nil, fmt.Errorf("no translator for detected package manager %q", pm)
		}
	case "windows":
		pm, err := engine.DetectWindowsPM(ctx, r)
		if err != nil {
			return nil, err
		}
		switch pm {
		case "winget":
			return translators.NewWinget(r)
		case "scoop":
			return translators.NewScoop(r)
		case "choco":
			return translators.NewChoco(r)
		default:
			return nil, fmt.Errorf("no translator for detected package manager %q", pm)
		}
	default:
		return nil, fmt.Errorf("unsupported OS %q", runtime.GOOS)
	}
}

func run(cmd *cobra.Command, _ []string) error {
	logger, err := newLogger()
	if err != nil {
		return err
	}
	slog.SetDefault(logger)

	profile, err := loadProfile(profilePath)
	if err != nil {
		return err
	}

	tr, err := translatorFactory(cmd.Context(), engine.ExecRunner{})
	if err != nil {
		return err
	}

	steps, err := engine.BuildPlan(profile, tr, hardening.Factory(engine.ExecRunner{}))
	if err != nil {
		return err
	}

	if dryRun {
		return engine.PrintPlan(cmd.OutOrStdout(), steps)
	}

	terminal := stdinTerminal()
	if !nonInteractive && !yes && !terminal {
		return errors.New("refusing to run non-interactively without --non-interactive or --yes")
	}

	if !yes && terminal {
		// Interactive session on a real terminal: drive the run from the TUI.
		// Any elevated step gets a sudo -v pre-auth before the TUI starts so
		// the whole TUI is never run as root.
		if needsSudo(steps) && !engine.IsElevated() && runtime.GOOS != "windows" {
			if err := preAuthSudo(); err != nil {
				return err
			}
		}
		store, err := newStore()
		if err != nil {
			return err
		}
		return ui.Run(cmd.Context(), ui.Options{
			Profile:  profilePath,
			Steps:    steps,
			Store:    store,
			Logger:   logger,
			Elevated: engine.IsElevated(),
		})
	}

	if !yes {
		ok, err := confirm(cmd, steps, cmd.InOrStdin())
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("aborted by user")
		}
	}

	store, err := newStore()
	if err != nil {
		return err
	}
	results := engine.NewExecutor(engine.ExecutorOptions{Logger: logger, Store: store}).Execute(cmd.Context(), steps)
	return summarize(cmd, results)
}

func auditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "audit",
		Short: "Report current state of every step without applying anything",
		RunE: func(cmd *cobra.Command, _ []string) error {
			profile, err := loadProfile(profilePath)
			if err != nil {
				return err
			}
			tr, err := translatorFactory(cmd.Context(), engine.ExecRunner{})
			if err != nil {
				return err
			}
			steps, err := engine.BuildPlan(profile, tr, hardening.Factory(engine.ExecRunner{}))
			if err != nil {
				return err
			}

			counts := map[engine.State]int{}
			for _, s := range steps {
				state, err := s.Check(cmd.Context())
				if err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "%-6s %s\n", "error", s.ID())
					continue
				}
				counts[state]++
				fmt.Fprintf(cmd.OutOrStdout(), "%-6s %s\n", state.String(), s.ID())
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d steps: %d satisfied, %d missing, %d drifted (audit only, no changes made)\n",
				len(steps), counts[engine.Satisfied], counts[engine.Missing], counts[engine.Drifted])
			return nil
		},
	}
}

func newLogger() (*slog.Logger, error) {
	var level slog.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	case "info", "":
		level = slog.LevelInfo
	default:
		return nil, fmt.Errorf("invalid log level %q", logLevel)
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})), nil
}

func newStore() (engine.Store, error) {
	if statePath == "" {
		p, err := engine.DefaultStatePath()
		if err != nil {
			return nil, err
		}
		statePath = p
	}
	return engine.NewJSONStore(statePath), nil
}

// stdinTerminal reports whether the process stdin is a character device. It is
// a package variable so tests can force either side regardless of the harness.
var stdinTerminal = func() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// needsSudo reports whether any step requires elevation.
func needsSudo(steps []engine.Step) bool {
	for _, s := range steps {
		if s.NeedsPrivilege() {
			return true
		}
	}
	return false
}

// preAuthSudo caches sudo credentials before the TUI starts so backgrounds
// inside the TUI do not prompt mid-render.
func preAuthSudo() error {
	if _, err := (engine.ExecRunner{}).Run(context.Background(), "sudo", "-v"); err != nil {
		return fmt.Errorf("sudo pre-auth failed: %w", err)
	}
	return nil
}

func confirm(cmd *cobra.Command, steps []engine.Step, in io.Reader) (bool, error) {
	if err := engine.PrintPlan(cmd.OutOrStdout(), steps); err != nil {
		return false, err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Apply these changes? [y/N] ")
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, os.ErrClosed) && !errors.Is(err, io.EOF) {
		return false, err
	}
	return strings.TrimSpace(line) == "y" || strings.TrimSpace(line) == "Y", nil
}

func summarize(cmd *cobra.Command, results []engine.StepResult) error {
	var applied, satisfied, failed, skipped int
	for _, r := range results {
		switch {
		case r.Err != nil:
			failed++
			fmt.Fprintf(cmd.OutOrStdout(), "FAILED   %s: %v\n", r.Step.ID(), r.Err)
		case r.CheckErr != nil:
			skipped++
			fmt.Fprintf(cmd.OutOrStdout(), "SKIPPED  %s: %v\n", r.Step.ID(), r.CheckErr)
		case r.Applied:
			applied++
			fmt.Fprintf(cmd.OutOrStdout(), "APPLIED  %s\n", r.Step.ID())
		default:
			satisfied++
			fmt.Fprintf(cmd.OutOrStdout(), "OK       %s (already satisfied)\n", r.Step.ID())
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\n%d steps: %d satisfied, %d applied, %d skipped, %d failed\n",
		len(results), satisfied, applied, skipped, failed)
	if failed > 0 {
		return fmt.Errorf("%d step(s) failed", failed)
	}
	return nil
}

// loadProfile resolves a profile from an existing file, or an embedded name.
func loadProfile(name string) (*config.Profile, error) {
	if name == "" {
		name = "dev.yaml"
	}
	if _, err := os.Stat(name); err == nil {
		return config.Load(name)
	}
	if data, err := profiles.Get(name); err == nil {
		return config.Parse(data)
	}
	return config.Load(name)
}
