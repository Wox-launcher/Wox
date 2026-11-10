if [[ -r ~/.bashrc ]]; then
    builtin source ~/.bashrc
fi

if [[ -n ${WOX_SHELL_SCROLLBACK-} ]]; then
    command cat -- "$WOX_SHELL_SCROLLBACK"
fi
unset WOX_SHELL_SCROLLBACK

__wox_history_entry() {
    HISTTIMEFORMAT= builtin history 1
}

__wox_command_started() {
    local entry number title
    entry=$(__wox_history_entry)
    [[ -n $entry && $entry != "${__wox_previous_history-}" ]] || return 0
    entry=${entry#"${entry%%[![:space:]]*}"}
    number=${entry%%[[:space:]]*}
    title=${entry#"$number"}
    builtin printf '\033]633;Wox;@WOX_TOKEN@;start;%s\007' "${title//[[:cntrl:]]/ }" >/dev/tty
}

__wox_command_finished() {
    local status=$?
    builtin printf '\033]633;Wox;@WOX_TOKEN@;end;%s\007' "$status"
    return "$status"
}

__wox_prompt_ready() {
    local status=$?
    __wox_previous_history=$(__wox_history_entry)
    __wox_waiting_for_command=1
    return "$status"
}

if (( BASH_VERSINFO[0] > 4 || (BASH_VERSINFO[0] == 4 && BASH_VERSINFO[1] >= 4) )); then
    PS0+='$(__wox_command_started)'
elif [[ -z $(builtin trap -p DEBUG) ]]; then
    # The Bash shipped with macOS predates PS0.
    __wox_debug() {
        if [[ ${__wox_waiting_for_command-} == 1 && $BASH_COMMAND != __wox_* ]]; then
            __wox_waiting_for_command=0
            __wox_command_started
        fi
    }
    builtin trap '__wox_debug' DEBUG
else
    return
fi

if [[ $(builtin declare -p PROMPT_COMMAND 2>/dev/null) == 'declare -a '* ]]; then
    PROMPT_COMMAND=(__wox_command_finished "${PROMPT_COMMAND[@]}" __wox_prompt_ready)
else
    PROMPT_COMMAND=$'__wox_command_finished\n'"${PROMPT_COMMAND-}"$'\n__wox_prompt_ready'
fi
