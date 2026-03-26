package cmd

import (
	"fmt"
	"os"
)

type CompletionCmd struct {
	Bash CompletionBashCmd `cmd:"" help:"Generate bash completions"`
	Zsh  CompletionZshCmd  `cmd:"" help:"Generate zsh completions"`
	Fish CompletionFishCmd `cmd:"" help:"Generate fish completions"`
}

type CompletionBashCmd struct{}

func (c *CompletionBashCmd) Run() error {
	script := `_frontcli_completions() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local commands="version config auth conversations messages drafts signatures tags inboxes teammates contacts channels comments templates completion whoami"

    if [ $COMP_CWORD -eq 1 ]; then
        COMPREPLY=($(compgen -W "$commands" -- "$cur"))
    fi
}

complete -F _frontcli_completions frontcli
`
	fmt.Fprint(os.Stdout, script)

	return nil
}

type CompletionZshCmd struct{}

func (c *CompletionZshCmd) Run() error {
	script := `#compdef frontcli

_frontcli() {
    local -a commands
    commands=(
        'version:Print version'
        'config:Manage configuration'
        'auth:Authentication and credentials'
        'conversations:Conversations'
        'messages:Messages'
        'drafts:Drafts'
        'signatures:Signatures'
        'tags:Tags'
        'inboxes:Inboxes'
        'teammates:Teammates'
        'contacts:Contacts'
        'channels:Channels'
        'comments:Comments'
        'templates:Templates'
        'completion:Generate shell completions'
        'whoami:Show authenticated user info'
    )

    _arguments \
        '1: :->command' \
        '*::arg:->args'

    case $state in
        command)
            _describe 'command' commands
            ;;
    esac
}

compdef _frontcli frontcli
`
	fmt.Fprint(os.Stdout, script)

	return nil
}

type CompletionFishCmd struct{}

func (c *CompletionFishCmd) Run() error {
	script := `complete -c frontcli -f

complete -c frontcli -n '__fish_use_subcommand' -a 'version' -d 'Print version'
complete -c frontcli -n '__fish_use_subcommand' -a 'config' -d 'Manage configuration'
complete -c frontcli -n '__fish_use_subcommand' -a 'auth' -d 'Authentication and credentials'
complete -c frontcli -n '__fish_use_subcommand' -a 'conversations' -d 'Conversations'
complete -c frontcli -n '__fish_use_subcommand' -a 'messages' -d 'Messages'
complete -c frontcli -n '__fish_use_subcommand' -a 'drafts' -d 'Drafts'
complete -c frontcli -n '__fish_use_subcommand' -a 'signatures' -d 'Signatures'
complete -c frontcli -n '__fish_use_subcommand' -a 'tags' -d 'Tags'
complete -c frontcli -n '__fish_use_subcommand' -a 'inboxes' -d 'Inboxes'
complete -c frontcli -n '__fish_use_subcommand' -a 'teammates' -d 'Teammates'
complete -c frontcli -n '__fish_use_subcommand' -a 'contacts' -d 'Contacts'
complete -c frontcli -n '__fish_use_subcommand' -a 'channels' -d 'Channels'
complete -c frontcli -n '__fish_use_subcommand' -a 'comments' -d 'Comments'
complete -c frontcli -n '__fish_use_subcommand' -a 'templates' -d 'Templates'
complete -c frontcli -n '__fish_use_subcommand' -a 'completion' -d 'Generate shell completions'
complete -c frontcli -n '__fish_use_subcommand' -a 'whoami' -d 'Show authenticated user info'
`
	fmt.Fprint(os.Stdout, script)

	return nil
}
