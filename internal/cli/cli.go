package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"

	"easy-cli/internal/app"
	"easy-cli/internal/config"
	"easy-cli/internal/provider"
	"easy-cli/internal/repl"
	"easy-cli/internal/ui"
	"easy-cli/internal/version"
)

// Options customizes command construction (tests inject IO and factories).
type Options struct {
	Getenv     func(string) string
	ConfigHome string
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	NewLLM     func(*config.Config) (provider.Provider, error)
}

func (o Options) getenv() func(string) string {
	if o.Getenv != nil {
		return o.Getenv
	}
	return os.Getenv
}

func (o Options) stdout() io.Writer {
	if o.Stdout != nil {
		return o.Stdout
	}
	return os.Stdout
}

func (o Options) stderr() io.Writer {
	if o.Stderr != nil {
		return o.Stderr
	}
	return os.Stderr
}

func (o Options) stdin() io.Reader {
	if o.Stdin != nil {
		return o.Stdin
	}
	return os.Stdin
}

func (o Options) configHome() string {
	if o.ConfigHome != "" {
		return o.ConfigHome
	}
	return xdg.ConfigHome
}

func (o Options) newLLM() func(*config.Config) (provider.Provider, error) {
	if o.NewLLM != nil {
		return o.NewLLM
	}
	return provider.New
}

// NewRoot builds the `easy` command tree.
func NewRoot(opts Options) *cobra.Command {
	var (
		scopes       []string
		providerName string
		jsonOut      bool
		yes          bool
	)

	root := &cobra.Command{
		Use:   "easy",
		Short: "Interactive read-only AI file search for Ubuntu",
		Long:  "easy plans a local filesystem search from a natural-language request, then uses Gemini, ChatGPT, or Claude only to rank a small snippet pack.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := buildApp(opts, providerName, yes, false)
			if err != nil {
				return err
			}
			return repl.Run(repl.IO{
				In:          opts.stdin(),
				Out:         opts.stdout(),
				Err:         opts.stderr(),
				App:         a,
				Scopes:      append([]string{}, scopes...),
				ConfigHome:  opts.configHome(),
				AutoConfirm: a.AutoConfirm,
			})
		},
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.String(),
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.PersistentFlags().StringSliceVar(&scopes, "scope", nil, "directories to search (default $HOME)")
	root.PersistentFlags().StringVar(&providerName, "provider", "", "openai, anthropic, or gemini")
	root.PersistentFlags().BoolVar(&jsonOut, "json", false, "print machine-readable JSON")
	root.PersistentFlags().BoolVarP(&yes, "yes", "y", false, "send file snippets to the cloud without prompting")

	root.SetOut(opts.stdout())
	root.SetErr(opts.stderr())
	root.AddCommand(newSearchCmd(opts, &scopes, &providerName, &jsonOut, &yes))
	root.AddCommand(newConfigCmd(opts, &providerName))
	root.AddCommand(newVersionCmd())
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), version.String())
			return nil
		},
	}
}

func newSearchCmd(opts Options, scopes *[]string, providerName *string, jsonOut *bool, yes *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "One-shot natural-language search",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := buildApp(opts, *providerName, *yes, true)
			if err != nil {
				return err
			}
			query := strings.Join(args, " ")
			var progress app.Progress
			if !*jsonOut {
				spin := ui.NewSpinner(opts.stdout())
				defer spin.Stop()
				progress = spin.AsProgress()
			}
			sess, err := a.SearchWithProgress(cmd.Context(), query, *scopes, progress)
			if err != nil {
				return err
			}
			if *jsonOut {
				enc := json.NewEncoder(opts.stdout())
				enc.SetIndent("", "  ")
				return enc.Encode(sess.Ranked)
			}
			printSession(opts.stdout(), sess)
			return nil
		},
	}
}

func newConfigCmd(opts Options, providerName *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show resolved configuration (API key redacted)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadCfg(opts, *providerName)
			if err != nil {
				return err
			}
			fmt.Fprintf(opts.stdout(), "provider: %s\nmodel: %s\nconfig: %s\ntimeout: %ds\nmax_files: %d\nmax_rank: %d\nauto_confirm: %v\nkey: %s\n",
				cfg.Provider, cfg.Model, cfg.ConfigPath, cfg.TimeoutSec, cfg.MaxFiles, cfg.MaxRank, cfg.AutoConfirm, redact(cfg.APIKey))
			return nil
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Write example config.toml (no API keys)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.WriteExample(opts.configHome())
			if err != nil {
				return err
			}
			fmt.Fprintf(opts.stdout(), "wrote %s\nexport GEMINI_API_KEY / OPENAI_API_KEY / ANTHROPIC_API_KEY in the environment.\n", path)
			return nil
		},
	})
	return cmd
}

func buildApp(opts Options, providerFlag string, yes bool, attachConfirm bool) (*app.App, error) {
	cfg, err := loadCfg(opts, providerFlag)
	if err != nil {
		return nil, err
	}
	llm, err := opts.newLLM()(cfg)
	if err != nil {
		return nil, err
	}
	a := app.New(cfg, llm)
	if yes {
		a.AutoConfirm = true
	}
	priv, err := config.LoadPrivacy(opts.configHome())
	if err != nil {
		return nil, err
	}
	a.PrivacyOK = priv.CloudSnippets
	if !a.AutoConfirm && attachConfirm {
		a.Confirm = makeConfirmer(opts)
	}
	return a, nil
}

func makeConfirmer(opts Options) app.Confirmer {
	in, inTTY := opts.stdin().(*os.File)
	out, outTTY := opts.stdout().(*os.File)
	interactive := inTTY && outTTY && ui.IsTTY(in) && ui.IsTTY(out)
	if !interactive {
		return func(context.Context, app.ConfirmRequest) (bool, error) {
			return false, app.ErrNeedYes
		}
	}
	return func(ctx context.Context, req app.ConfirmRequest) (bool, error) {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		fmt.Fprintln(opts.stdout(), ui.RenderConfirm(req))
		fmt.Fprint(opts.stdout(), "send snippets to the cloud? [y/N] ")
		line, err := bufio.NewReader(opts.stdin()).ReadString('\n')
		if err != nil && len(strings.TrimSpace(line)) == 0 {
			return false, nil
		}
		ans := strings.ToLower(strings.TrimSpace(line))
		ok := ans == "y" || ans == "yes"
		if ok && req.FirstRun {
			_ = config.SavePrivacy(opts.configHome(), config.Privacy{
				CloudSnippets: true,
				AcceptedAt:    time.Now().UTC().Format(time.RFC3339),
			})
		}
		return ok, nil
	}
}

func loadCfg(opts Options, providerFlag string) (*config.Config, error) {
	getenv := opts.getenv()
	if providerFlag != "" {
		inner := getenv
		getenv = func(k string) string {
			if k == "EASY_PROVIDER" {
				return providerFlag
			}
			return inner(k)
		}
	}
	return config.LoadFrom(opts.configHome(), getenv)
}

func redact(key string) string {
	if key == "" {
		return "(missing)"
	}
	if len(key) <= 6 {
		return "******"
	}
	return key[:3] + "…" + key[len(key)-2:]
}

func printSession(w io.Writer, sess *app.Session) {
	fmt.Fprintln(w, ui.RenderSession(sess))
}
