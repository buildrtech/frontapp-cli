package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

type RootFlags struct {
	Account string `help:"Account email for multi-account support"`
	Client  string `help:"OAuth client name override"`
	JSON    bool   `help:"Output JSON to stdout (best for scripting)"`
	Plain   bool   `help:"Output TSV (stable for scripts)"`
	Verbose bool   `help:"Enable verbose logging"`
}

type CLI struct {
	RootFlags `embed:""`

	Version    kong.VersionFlag `help:"Print version and exit"`
	VersionCmd VersionCmd       `cmd:"" name:"version" help:"Print version"`
	Config     ConfigCmd        `cmd:"" help:"Manage configuration"`
	Auth       AuthCmd          `cmd:"" help:"Authentication and credentials"`
	Conv       ConvCmd          `cmd:"" name:"conversations" aliases:"conv" help:"Conversations"`
	Msg        MsgCmd           `cmd:"" name:"messages" aliases:"msg" help:"Messages"`
	Draft      DraftCmd         `cmd:"" name:"drafts" help:"Drafts"`
	Signature  SignatureCmd     `cmd:"" name:"signatures" help:"Signatures"`
	Tag        TagCmd           `cmd:"" name:"tags" help:"Tags"`
	Inbox      InboxCmd         `cmd:"" name:"inboxes" help:"Inboxes"`
	Teammate   TeammateCmd      `cmd:"" name:"teammates" help:"Teammates"`
	Contact    ContactCmd       `cmd:"" name:"contacts" help:"Contacts"`
	Channel    ChannelCmd       `cmd:"" name:"channels" help:"Channels"`
	Comment    CommentCmd       `cmd:"" name:"comments" help:"Comments (internal discussions)"`
	Template   TemplateCmd      `cmd:"" name:"templates" help:"Templates (canned responses)"`
	Completion CompletionCmd    `cmd:"" help:"Generate shell completions"`
	Whoami     WhoamiCmd        `cmd:"" help:"Show authenticated user info"`
}

type exitPanic struct{ code int }

func Execute(args []string) (err error) {
	parser, err := newParser()
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			if ep, ok := r.(exitPanic); ok {
				if ep.code == 0 {
					err = nil

					return
				}

				err = &ExitError{Code: ep.code, Err: errors.New("exited")}

				return
			}

			panic(r)
		}
	}()

	// Show help when no command provided
	if len(args) == 0 {
		args = []string{"--help"}
	}

	kctx, err := parser.Parse(args)
	if err != nil {
		parsedErr := wrapParseError(err)
		_, _ = fmt.Fprintln(os.Stderr, parsedErr)

		return parsedErr
	}

	err = kctx.Run()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)

		return err
	}

	return nil
}

func wrapParseError(err error) error {
	if err == nil {
		return nil
	}

	var parseErr *kong.ParseError
	if errors.As(err, &parseErr) {
		return &ExitError{Code: 2, Err: parseErr}
	}

	return err
}

func newParser() (*kong.Kong, error) {
	vars := kong.Vars{
		"version": VersionString(),
	}

	cli := &CLI{}
	parser, err := kong.New(
		cli,
		kong.Name("frontcli"),
		kong.Description("Front CLI - interact with FrontApp from the command line"),
		kong.Vars(vars),
		kong.Writers(os.Stdout, os.Stderr),
		kong.Exit(func(code int) { panic(exitPanic{code: code}) }),
		kong.Bind(&cli.RootFlags),
		kong.Help(helpPrinter),
		kong.ConfigureHelp(helpOptions()),
	)
	if err != nil {
		return nil, err
	}

	return parser, nil
}
