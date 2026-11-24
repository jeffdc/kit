// Personal safety guards for OMP, installed by ../install.sh as
// ~/.omp/agent/hooks/pre/guards.ts.
//
// Ports the Claude Code deny/ask lists and the global CLAUDE.md git rules.
// "deny" always blocks. "ask" prompts in the TUI and blocks when there is no UI
// (print mode, subagents), so a headless agent can never take an ask-level action.
// A thrown error also blocks: OMP fails closed on tool_call handler errors.
//
// Directories the agent may never touch are personal, so they are not in this
// file. They live in ~/.omp/agent/guards.json: { "protectedDirs": ["~/.private"] }.
// A missing or unreadable guards.json means no protected directories.

import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import type { HookAPI } from "@oh-my-pi/pi-coding-agent/extensibility/hooks";

type Verdict = { level: "deny" | "ask"; reason: string };
type Exec = (command: string, args: string[], options?: { cwd?: string; timeout?: number }) => Promise<{ stdout: string; code: number }>;

// ---------------------------------------------------------------------------
// Shell text checks (bash commands, and shell text inside eval code)

// A command word must sit at a command position: start, after a separator,
// after a subshell/backtick opener, at the start of a quoted string (so shell
// text inside eval code or `bash -c '...'` is checked), or after a wrapper such as sudo or xargs.
const CMD_START = String.raw`(?:^|[;&|({\n\x60'"]|\$\(|\b(?:sudo|xargs|exec|command|nohup|time|then|do|else)\s)\s*`;
const cmd = (words: string) => new RegExp(`${CMD_START}(?:\\S*/)?(?:${words})(?=\\s|$|[;&|'")])`, "m");
// git, optionally with global flags (-C dir, -c k=v, --no-pager...) before the subcommand.
const git = (sub: string) => new RegExp(`${CMD_START}git(?:\\s+(?:-[Cc]\\s+\\S+|--?[\\w-]+(?:=\\S+)?))*\\s+(?:${sub})`, "m");

const SECRET_FILE = String.raw`(?:^|[\s'"=:/<>])(?:\.env(?:\.(?!example\b|sample\b|template\b|dist\b)[\w.-]+)?|\.envrc\.local)(?=$|[\s'";|&)>])`;
const SECRET_FILE_RE = new RegExp(SECRET_FILE, "m");

// ---------------------------------------------------------------------------
// Protected directories (from guards.json)

const CONFIG_PATH = path.join(os.homedir(), ".omp", "agent", "guards.json");

function parseProtectedDirs(json: string): string[] {
	const dirs = (JSON.parse(json) as { protectedDirs?: unknown }).protectedDirs;
	return Array.isArray(dirs) ? dirs.filter((d): d is string => typeof d === "string" && d.trim() !== "") : [];
}

function loadProtectedDirs(file = CONFIG_PATH): string[] {
	try {
		return parseProtectedDirs(fs.readFileSync(file, "utf8"));
	} catch {
		return [];
	}
}

const escapeRe = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

// Matches a protected directory however a path or command spells it: ~/d,
// $HOME/d, ${HOME}/d, the absolute path, or a bare relative d.
function protectedDirRe(dir: string, home = os.homedir()): RegExp {
	const abs = dir.replace(/^~(?=\/|$)/, home).replace(/\/+$/, "");
	const rel = abs.startsWith(`${home}/`) ? abs.slice(home.length + 1) : undefined;
	const spelled = rel ? `(?:~/|\\$HOME/|\\$\\{HOME\\}/|${escapeRe(home)}/)?${escapeRe(rel)}` : escapeRe(abs);
	return new RegExp(`(?:^|[\\s'"=:(\x60])${spelled}(?=/|$|[\\s'";|&)\x60])`, "m");
}

const PROTECTED_REASON = "this directory is off limits (guards.json)";

const SHELL_DENY: Array<[RegExp, string]> = [
	[git(String.raw`add\b[^;&|\n]*(?:\s(?:-A|--all|-u|--update|\.|:\/|\*)(?=\s|$|[;&|'")]))`), "git add -A/./-u is not allowed; stage specific files by name"],
	[git(String.raw`commit\b[^;&|\n]*\s-[a-zA-Z]*a`), "git commit -a is not allowed; stage specific files by name"],
	[git(String.raw`push\b[^;&|\n]*(?:\s(?:-f|--force(?:-with-lease|-if-includes)?(?:=\S*)?|--mirror)(?=\s|$|[;&|'")])|\s\+\S)`), "force push is not allowed"],
	[git(String.raw`push\b[^;&|\n]*(?:\s|:)(?:main|master)(?=\s|$|[;&|'")])`), "pushing to main/master is not allowed"],
	[cmd("printenv"), "printing the environment is not allowed (secrets)"],
	[new RegExp(`${CMD_START}(?:env|set|export|export\\s+-p|declare\\s+-[a-zA-Z]*[xp][a-zA-Z]*|typeset\\s+-[a-zA-Z]*x|compgen\\s+-[ev])\\s*(?=$|[;&|>)])`, "m"), "printing the environment is not allowed (secrets)"],
	[new RegExp(`${CMD_START}(?:\\/bin\\/)?ps\\b[^;&|\\n]*[eE]`, "m"), "ps with environment output is not allowed (secrets)"],
	[new RegExp(`${CMD_START}direnv\\s+(?:export|dump|show_dump)\\b`, "m"), "direnv export prints secret values"],
	[SECRET_FILE_RE, ".env files may not be read or touched by the agent"],
];

const SHELL_ASK: Array<[RegExp, string]> = [
	[cmd("rm|rmdir|unlink|shred|srm|trash|mv|truncate|dd"), "delete/move/overwrite command"],
	[new RegExp(String.raw`\bfind\b[^;&|\n]*\s-(?:delete|exec\s+rm)\b`, "m"), "find -delete"],
	[new RegExp(String.raw`\brsync\b[^;&|\n]*\s--delete`, "m"), "rsync --delete"],
	[cmd("kill|pkill|killall"), "kill a process"],
	[git(String.raw`(?:(?:rm|mv|clean|rebase|filter-branch|filter-repo|restore)(?=\s|$|[;&|'")])|switch\s+(?:-f|--discard-changes)|checkout\s+(?:-f\b|--\s|\.(?:\s|$))|reset\s+[^;&|\n]*--hard|stash\s+(?:drop|clear)|branch\s+[^;&|\n]*-[dD]\b|tag\s+[^;&|\n]*-d\b|worktree\s+remove|update-ref\s+-d|reflog\s+expire|gc\s+[^;&|\n]*--prune)`), "destructive git command"],
	[git("push"), "git push publishes commits"],
	[new RegExp(String.raw`${CMD_START}gh\s+(?:pr\s+(?:create|merge|comment|edit|close|review)|issue\s+(?:create|comment|edit|close)|release|gist|repo\s+(?:create|delete|sync|edit)|api\b[^;&|\n]*-X\s*(?:POST|PATCH|PUT|DELETE))`, "m"), "gh command publishes to GitHub"],
	[new RegExp(String.raw`\bmix\s+ecto\.(?:drop|reset|rollback)\b|${CMD_START}dropdb\b|\b(?:DROP\s+(?:TABLE|DATABASE|SCHEMA)|TRUNCATE)\b`, "mi"), "destructive database command"],
];

function shellDeny(text: string): Verdict | undefined {
	for (const [re, reason] of SHELL_DENY) if (re.test(text)) return { level: "deny", reason };
	return undefined;
}

function shellAsk(text: string): Verdict | undefined {
	for (const [re, reason] of SHELL_ASK) if (re.test(text)) return { level: "ask", reason };
	return undefined;
}

const checkShell = (text: string) => shellDeny(text) ?? shellAsk(text);

// Bare `git push` (no explicit refspec) while on main/master, and amending a
// commit that is already on a remote branch. Both need repository state.
async function checkGitState(command: string, cwd: string, exec: Exec): Promise<Verdict | undefined> {
	const run = async (args: string[]) => {
		const r = await exec("git", args, { cwd, timeout: 5000 });
		return r.code === 0 ? r.stdout.trim() : undefined;
	};
	if (git("push").test(command)) {
		const branch = await run(["rev-parse", "--abbrev-ref", "HEAD"]);
		if (branch === "main" || branch === "master") return { level: "deny", reason: `current branch is ${branch}; pushing to main/master is not allowed` };
	}
	if (git(String.raw`commit\b[^;&|\n]*--amend`).test(command)) {
		const remotes = await run(["branch", "-r", "--contains", "HEAD"]);
		if (remotes === undefined) return { level: "ask", reason: "amend: could not check whether HEAD is pushed" };
		if (remotes !== "") return { level: "deny", reason: "HEAD is already pushed; amending pushed commits is not allowed" };
	}
	return undefined;
}

// ---------------------------------------------------------------------------
// Per-tool checks

function strings(value: unknown, out: string[] = []): string[] {
	if (typeof value === "string") out.push(value);
	else if (Array.isArray(value)) for (const v of value) strings(v, out);
	else if (value && typeof value === "object") for (const v of Object.values(value)) strings(v, out);
	return out;
}

// Path-like arguments of tools (bash takes its own cwd). grep/glob accept "a; b" lists.
function pathArgs(input: Record<string, unknown>): string[] {
	return ["path", "paths", "rename", "cwd"].flatMap((k) => strings(input[k])).flatMap((p) => p.split(";")).map((p) => p.trim()).filter(Boolean);
}

const isSecretPath = (p: string) => SECRET_FILE_RE.test(` ${p.replace(/:\d.*$/, "")}`);
const inProtected = (text: string, protectedRes: RegExp[]) => protectedRes.some((re) => re.test(` ${text}`));

// Edit tool: patch mode (`edits[].op`/`rename`), apply_patch, and hashline (`REM`, `MV`).
function editDeletesOrMoves(input: Record<string, unknown>): boolean {
	const edits = Array.isArray(input.edits) ? (input.edits as Array<Record<string, unknown>>) : [];
	if (edits.some((e) => e?.op === "delete" || typeof e?.rename === "string")) return true;
	const text = typeof input.input === "string" ? input.input : "";
	return /^\s*(?:REM|MV\s+\S.*)\s*$/m.test(text) || /^\*\*\* (?:Delete File|Move to):/m.test(text);
}

const EVAL_FS_DESTRUCTIVE = /\b(?:os\.(?:remove|unlink|rmdir|removedirs|rename|renames|replace)|shutil\.(?:rmtree|move)|\.unlink\(|\.rmdir\(|\.rename\(|\.replace\([^)]*Path|unlinkSync|rmSync|rmdirSync|renameSync|fs\.(?:rm|unlink|rmdir|rename)\b|Bun\.file\([^)]*\)\.delete)/;
const EVAL_SUBPROCESS = /\b(?:subprocess|os\.system|os\.popen|os\.exec\w*|pty\.spawn|child_process|execSync|spawnSync|Bun\.spawn|Bun\.\$)|\$`/;
const EVAL_ENV = /\b(?:os\.environ|os\.getenv|process\.env|Bun\.env|import\.meta\.env)\b/;

async function evaluate(
	toolName: string,
	input: Record<string, unknown>,
	cwd: string,
	exec: Exec,
	protectedRes: RegExp[] = [],
): Promise<Verdict | undefined> {
	// Only paths, commands and code are checked for protected directories, so a
	// file whose content merely mentions one (such as AGENTS.md) can still be written.
	const isProtected = (text: string) => inProtected(text, protectedRes);
	if (isProtected(cwd) || pathArgs(input).some(isProtected)) return { level: "deny", reason: PROTECTED_REASON };

	switch (toolName) {
		case "bash": {
			const command = String(input.command ?? "");
			if (isProtected(command)) return { level: "deny", reason: PROTECTED_REASON };
			const runCwd = typeof input.cwd === "string" && input.cwd ? path.resolve(cwd, input.cwd.replace(/^~(?=\/|$)/, os.homedir())) : cwd;
			return shellDeny(command) ?? (await checkGitState(command, runCwd, exec)) ?? shellAsk(command);
		}
		case "eval": {
			const code = String(input.code ?? "");
			if (isProtected(code)) return { level: "deny", reason: PROTECTED_REASON };
			const shell = checkShell(code);
			if (shell?.level === "deny") return shell;
			if (EVAL_ENV.test(code)) return { level: "ask", reason: "eval reads environment variables (secrets)" };
			if (EVAL_FS_DESTRUCTIVE.test(code)) return { level: "ask", reason: "eval deletes or moves files" };
			if (EVAL_SUBPROCESS.test(code)) return { level: "ask", reason: "eval starts a subprocess" };
			return shell;
		}
		case "edit":
			if (pathArgs(input).some(isSecretPath)) return { level: "deny", reason: ".env files may not be read or touched by the agent" };
			if (editDeletesOrMoves(input)) return { level: "ask", reason: "edit deletes or moves a file" };
			if (typeof input.input === "string") {
				// Patch header lines name the files: "*** Update File: p" (apply_patch), "[p#TAG]" (hashline).
				const headers = input.input.split("\n").filter((l) => /^(?:\*\*\*|\[)/.test(l)).map((l) => l.replace(/^\*\*\* [\w ]+:\s*|^\[|#[^\]]*\]$|\]$/g, ""));
				if (headers.some(isSecretPath)) return { level: "deny", reason: ".env files may not be read or touched by the agent" };
				if (headers.some(isProtected)) return { level: "deny", reason: PROTECTED_REASON };
			}
			return undefined;
		case "read":
		case "write":
		case "grep":
		case "glob":
		case "ast_grep":
		case "ast_edit":
			if (pathArgs(input).some(isSecretPath)) return { level: "deny", reason: ".env files may not be read or touched by the agent" };
			return undefined;
		case "github": {
			const op = String(input.op ?? "");
			if (op === "pr_push" || op === "pr_create") {
				const branch = String(input.branch ?? "");
				if (branch === "main" || branch === "master") return { level: "deny", reason: "pushing to main/master is not allowed" };
				return { level: "ask", reason: `github ${op} publishes to GitHub` };
			}
			return undefined;
		}
		default:
			return undefined;
	}
}

// ---------------------------------------------------------------------------

export default function guards(pi: HookAPI): void {
	// Read once per session; restart OMP after editing guards.json.
	const protectedRes = loadProtectedDirs().map((d) => protectedDirRe(d));
	pi.on("tool_call", async (event, ctx) => {
		const input = (event.input ?? {}) as Record<string, unknown>;
		const verdict = await evaluate(event.toolName, input, ctx.cwd, pi.exec.bind(pi), protectedRes);
		if (!verdict) return;
		if (verdict.level === "deny") return { block: true, reason: `guard: ${verdict.reason}` };
		if (!ctx.hasUI) return { block: true, reason: `guard: ${verdict.reason} needs approval, and this session has no UI` };
		const detail = event.toolName === "bash" ? String(input.command ?? "") : JSON.stringify(input, null, 2).slice(0, 2000);
		const ok = await ctx.ui.confirm(`Guard: ${verdict.reason}`, detail);
		if (!ok) return { block: true, reason: `guard: ${verdict.reason} (denied by user)` };
	});
}

// Exported for tests only.
export const _internal = { evaluate, checkShell, pathArgs, parseProtectedDirs, loadProtectedDirs, protectedDirRe };
