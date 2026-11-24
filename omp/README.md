# omp

Personal configuration for [OMP](https://github.com/can1357/oh-my-pi) (oh-my-pi), the coding agent harness.

## Guard hook

`hooks/guards.ts` is a `tool_call` hook that enforces the safety rules that used to live in Claude Code's permission settings. It checks `bash`, `eval`, `edit`, `read`, `write`, `grep`, `glob` and `github` calls.

- **Blocked:** `git add -A`/`.`/`-u`, `git commit -a`, force pushes, pushes to main/master, amending a pushed commit, printing the environment (`env`, `printenv`, `ps e`, `direnv export`), `.env` files, and protected directories.
- **Asks first:** deleting or moving files (shell, edit tool or eval code), killing processes, destructive git commands, `git push`, publishing `gh` commands, and destructive database commands. With no UI (print mode, subagents) these are blocked.

It is pattern matching, not a sandbox: an agent that writes a script and then runs it can get past it.

Protected directories are personal, so they are not in this repository. List them in `~/.omp/agent/guards.json`:

```json
{ "protectedDirs": ["~/.private"] }
```

OMP reads that file once per session.

## Install and test

- `make test` runs the hook's tests (Bun).
- `make install` copies the hook to `~/.omp/agent/hooks/pre/guards.ts`. OMP skips symlinks there, so it is a copy. After changing the hook, run `./install.sh --force` to replace the installed copy.
