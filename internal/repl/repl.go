// Package repl is the interactive loop. It depends on app, not on Cobra.
package repl

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"easy-cli/internal/app"
	"easy-cli/internal/ui"
)

// IO is the REPL environment.
type IO struct {
	In          io.Reader
	Out         io.Writer
	Err         io.Writer
	App         *app.App
	Scopes      []string
	ConfigHome  string
	AutoConfirm bool
}

// Run starts a boxed TUI on a real terminal, or a line REPL for pipes/tests.
func Run(env IO) error {
	if env.Out == nil {
		return fmt.Errorf("repl: nil stdout")
	}
	if env.In == nil {
		return fmt.Errorf("repl: nil stdin")
	}
	if env.Err == nil {
		env.Err = env.Out
	}
	if len(env.Scopes) == 0 {
		env.Scopes = app.DefaultRoots()
	}
	if env.App != nil {
		if env.AutoConfirm {
			env.App.AutoConfirm = true
		}
		if !env.App.AutoConfirm && env.App.Confirm == nil && !isInteractive(env) {
			env.App.Confirm = func(context.Context, app.ConfirmRequest) (bool, error) {
				return false, app.ErrNeedYes
			}
		}
	}
	if isInteractive(env) {
		return runTUI(&env)
	}
	return runLine(&env)
}

func isInteractive(env IO) bool {
	in, ok1 := env.In.(*os.File)
	out, ok2 := env.Out.(*os.File)
	return ok1 && ok2 && ui.IsTTY(in) && ui.IsTTY(out)
}

func runLine(env *IO) error {
	fmt.Fprintln(env.Out, ui.Header(providerName(env), strings.Join(env.Scopes, " ")))
	fmt.Fprintln(env.Out, ui.HintBar())
	sc := bufio.NewScanner(env.In)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for {
		fmt.Fprint(env.Out, "easy> ")
		if !sc.Scan() {
			fmt.Fprintln(env.Out)
			return sc.Err()
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if err := handle(env, line); err != nil {
			if err == errExit {
				return nil
			}
			fmt.Fprintln(env.Err, ui.RenderError(err))
		}
	}
}

var errExit = fmt.Errorf("exit")

func providerName(env *IO) string {
	if env != nil && env.App != nil && env.App.LLM != nil {
		return env.App.LLM.Name()
	}
	return "none"
}

func handle(env *IO, line string) error {
	ctx := context.Background()
	fmt.Fprintln(env.Out, ui.RenderYou(line))
	cmd, rest, _ := strings.Cut(line, " ")
	cmd = strings.ToLower(strings.TrimSpace(cmd))
	rest = strings.TrimSpace(rest)
	switch cmd {
	case "exit", "quit":
		return errExit
	case "help", "?":
		fmt.Fprintln(env.Out, ui.HelpText())
		return nil
	case "scope":
		if rest == "" {
			fmt.Fprintln(env.Out, ui.RenderScope(env.Scopes))
			return nil
		}
		env.Scopes = strings.Fields(rest)
		fmt.Fprintln(env.Out, ui.RenderScope(env.Scopes))
		return nil
	case "provider":
		fmt.Fprintf(env.Out, "provider is selected at startup via --provider or EASY_PROVIDER; current: %s\n", providerName(env))
		return nil
	case "find", "search":
		return runSearch(ctx, env, rest)
	default:
		if env.App != nil && env.App.Last != nil && isQuestion(line) {
			return runAsk(ctx, env, line)
		}
		return runSearch(ctx, env, line)
	}
}

func runSearch(ctx context.Context, env *IO, query string) error {
	if query == "" {
		return fmt.Errorf("empty query")
	}
	if env.App == nil {
		return fmt.Errorf("repl: nil app")
	}
	spin := ui.NewSpinner(env.Out)
	defer spin.Stop()
	sess, err := env.App.SearchWithProgress(ctx, query, env.Scopes, spin.AsProgress())
	if err != nil {
		return err
	}
	fmt.Fprintln(env.Out, ui.RenderSession(sess))
	return nil
}

func runAsk(ctx context.Context, env *IO, question string) error {
	spin := ui.NewSpinner(env.Out)
	defer spin.Stop()
	ans, err := env.App.AskWithProgress(ctx, question, spin.AsProgress())
	if err != nil {
		return err
	}
	fmt.Fprintln(env.Out, ui.RenderAnswer(ans))
	return nil
}

func isQuestion(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.Contains(s, "?") {
		return true
	}
	for _, p := range []string{"what ", "what's ", "how ", "who ", "summar", "explain", "tell me"} {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
