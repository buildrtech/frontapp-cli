package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/dedene/frontapp-cli/internal/errfmt"
	"github.com/dedene/frontapp-cli/internal/output"
)

type SignatureCmd struct {
	List SignatureListCmd `cmd:"" help:"List signatures for a teammate"`
	Get  SignatureGetCmd  `cmd:"" help:"Get a signature"`
}

type SignatureListCmd struct {
	Teammate string `help:"Teammate ID" required:""`
}

func (c *SignatureListCmd) Run(flags *RootFlags) error {
	ctx := context.Background()

	client, err := getClient(flags)
	if err != nil {
		return err
	}

	mode, err := resolveOutputMode(flags)
	if err != nil {
		return err
	}

	resp, err := client.ListTeammateSignatures(ctx, c.Teammate)
	if err != nil {
		fmt.Fprint(os.Stderr, errfmt.Format(err))

		return err
	}

	if mode.JSON {
		return output.WriteJSON(os.Stdout, resp)
	}

	if len(resp.Results) == 0 {
		fmt.Fprintln(os.Stdout, "No signatures found.")

		return nil
	}

	tbl := output.NewTableWriter(os.Stdout, mode.Plain)
	tbl.AddRow("ID", "NAME", "DEFAULT")

	for _, sig := range resp.Results {
		tbl.AddRow(
			sig.ID,
			sig.Name,
			fmt.Sprintf("%t", sig.IsDefault),
		)
	}

	return tbl.Flush()
}

type SignatureGetCmd struct {
	ID string `arg:"" help:"Signature ID"`
}

func (c *SignatureGetCmd) Run(flags *RootFlags) error {
	ctx := context.Background()

	client, err := getClient(flags)
	if err != nil {
		return err
	}

	mode, err := resolveOutputMode(flags)
	if err != nil {
		return err
	}

	sig, err := client.GetSignature(ctx, c.ID)
	if err != nil {
		fmt.Fprint(os.Stderr, errfmt.Format(err))

		return err
	}

	if mode.JSON {
		return output.WriteJSON(os.Stdout, sig)
	}

	fmt.Fprintf(os.Stdout, "ID:      %s\n", sig.ID)
	fmt.Fprintf(os.Stdout, "Name:    %s\n", sig.Name)
	fmt.Fprintf(os.Stdout, "Default: %t\n", sig.IsDefault)

	if sig.Body != "" {
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, sig.Body)
	}

	return nil
}
