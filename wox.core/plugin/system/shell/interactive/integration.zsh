# Restore the user's startup directory before sourcing their .zshenv.
if [[ $WOX_ZDOTDIR_SET == 1 ]]; then
    export ZDOTDIR=$WOX_ORIGINAL_ZDOTDIR
else
    unset ZDOTDIR
fi
unset WOX_ZDOTDIR_SET WOX_ORIGINAL_ZDOTDIR
if [[ -r ${ZDOTDIR-$HOME}/.zshenv ]]; then
    builtin source "${ZDOTDIR-$HOME}/.zshenv"
fi

if [[ -n ${WOX_SHELL_SCROLLBACK-} ]]; then
    command cat -- "$WOX_SHELL_SCROLLBACK"
fi
unset WOX_SHELL_SCROLLBACK

autoload -Uz add-zsh-hook
__wox_command_started() {
    local title=$1
    builtin printf '\033]633;Wox;@WOX_TOKEN@;start;%s\007' "${title//[[:cntrl:]]/ }"
}
__wox_command_finished() {
    local exit_code=$?
    builtin printf '\033]633;Wox;@WOX_TOKEN@;end;%s\007' "$exit_code"
    return 0
}
if [[ -o interactive ]]; then
    add-zsh-hook preexec __wox_command_started
    add-zsh-hook precmd __wox_command_finished
fi
