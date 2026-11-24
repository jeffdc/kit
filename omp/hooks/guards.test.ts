import { describe, expect, test } from "bun:test";
import guards, { _internal } from "./guards";

const { evaluate, parseProtectedDirs, loadProtectedDirs, protectedDirRe } = _internal;

// Fake git: branch and whether HEAD is on a remote branch.
function fakeExec(branch = "feature", pushed = false) {
	return async (_cmd: string, args: string[]) => {
		if (args[0] === "rev-parse") return { stdout: `${branch}\n`, code: 0 };
		if (args[0] === "branch") return { stdout: pushed ? "  origin/feature\n" : "", code: 0 };
		return { stdout: "", code: 1 };
	};
}

// Tests use a made-up home and protected directory, never real ones.
const HOME = "/home/u";
const CWD = `${HOME}/dev/project`;
const PROTECTED = [protectedDirRe("~/.private", HOME), protectedDirRe("/srv/secret", HOME)];
const level = async (tool: string, input: Record<string, unknown>, exec = fakeExec(), cwd = CWD) =>
	(await evaluate(tool, input, cwd, exec, PROTECTED))?.level ?? "allow";
const bash = (command: string, exec = fakeExec()) => level("bash", { command }, exec);

describe("bash deny", () => {
	test.each([
		"git add -A",
		"git add .",
		"git add --all",
		"git add -u",
		"git -C ../x add -A",
		"cd lib && git add .",
		"git commit -am 'wip'",
		"git push --force origin feature",
		"git push -f",
		"git push --force-with-lease",
		"git push origin +feature",
		"git push origin main",
		"git push origin HEAD:master",
		"printenv",
		"printenv HOME",
		"env",
		"env | grep KEY",
		"export",
		"export -p",
		"set",
		"declare -x",
		"declare -p",
		"ps eww 123",
		"ps auxe",
		"/bin/ps -E",
		"cat .env",
		"cat config/.env.local",
		"grep KEY .envrc.local",
		"cp .env /tmp/x",
		"direnv export json",
		"ls ~/.private",
		"cat /home/u/.private/notes",
		"cat $HOME/.private/x",
		"cd .private && ls",
		"ls /srv/secret/db",
		"echo $(cat .env)",
		"bash -c 'git add -A'",
	])("%s", async (c) => expect(await bash(c)).toBe("deny"));

	test("bare push on main", async () => expect(await bash("git push", fakeExec("main"))).toBe("deny"));
	test("bare push -u on master", async () => expect(await bash("git push -u origin HEAD", fakeExec("master"))).toBe("deny"));
	test("amend pushed commit", async () => expect(await bash("git commit --amend --no-edit", fakeExec("feature", true))).toBe("deny"));
});

describe("bash ask", () => {
	test.each([
		"rm foo.txt",
		"rm -rf _build",
		"/bin/rm x",
		"sudo rm x",
		"mv a b",
		"rmdir tmp",
		"ls | xargs rm",
		"find . -name '*.beam' -delete",
		"rsync -a --delete src/ dst/",
		"kill 123",
		"pkill beam",
		"killall node",
		"git rm lib/old.ex",
		"git mv a b",
		"git clean -fd",
		"git reset --hard HEAD~1",
		"git checkout -- lib/x.ex",
		"git checkout .",
		"git restore lib/x.ex",
		"git stash drop",
		"git branch -D old",
		"git rebase -i main",
		"git push",
		"git push -u origin feature",
		"gh pr create --fill",
		"gh issue comment 12 --body hi",
		"gh api repos/x/y/issues -X POST -f title=t",
		"mix ecto.drop",
		"mix ecto.reset",
		"psql -c 'DROP TABLE galls'",
	])("%s", async (c) => expect(await bash(c)).toBe("ask"));
});

describe("bash allow", () => {
	test.each([
		"mix test",
		"mix precommit",
		"make ci",
		"git status",
		"git diff",
		"git log --oneline -5",
		"git add lib/gallformers/galls.ex test/galls_test.exs",
		"git add ./lib/foo.ex",
		"git commit -m 'add a thing'",
		"git commit --amend --no-edit",
		"git branch -a",
		"git checkout -b new-feature",
		"git switch main",
		"docker run --rm postgres",
		"env MIX_ENV=test mix test",
		"MIX_ENV=test mix test",
		"export FOO=bar",
		"set -euo pipefail",
		"ps aux | grep beam",
		"cat .env.example",
		"source .envrc",
		"ls ~/dev/private",
		"ls ~/.private-other",
		"ls /srv/secretary",
		"echo format",
		"gh pr view 12",
		"gh pr list",
		"mix ecto.migrate",
		"grep -r 'process.env' assets/",
	])("%s", async (c) => expect(await bash(c)).toBe("allow"));
});

describe("file tools", () => {
	test("read .env denied", async () => expect(await level("read", { path: "/x/.env" })).toBe("deny"));
	test("read .env.prod denied", async () => expect(await level("read", { path: ".env.prod" })).toBe("deny"));
	test("read .env with selector denied", async () => expect(await level("read", { path: ".env:1-5" })).toBe("deny"));
	test("read .env.example allowed", async () => expect(await level("read", { path: ".env.example" })).toBe("allow"));
	test("read normal file allowed", async () => expect(await level("read", { path: "lib/gallformers.ex" })).toBe("allow"));
	test("grep list with .env denied", async () => expect(await level("grep", { pattern: "KEY", path: "lib; .env" })).toBe("deny"));
	test("glob protected dir denied", async () => expect(await level("glob", { path: "~/.private/**" })).toBe("deny"));
	test("write .env denied", async () => expect(await level("write", { path: ".env", content: "x" })).toBe("deny"));
	test("write doc mentioning protected dir allowed", async () =>
		expect(await level("write", { path: "AGENTS.md", content: "Never touch ~/.private or .env files." })).toBe("allow"));
	test("bash cwd argument inside protected dir denied", async () => expect(await level("bash", { command: "ls", cwd: "~/.private" })).toBe("deny"));
	test("bash cwd argument checked for bare push on main", async () => {
		const seen: string[] = [];
		const exec = async (_c: string, args: string[], o?: { cwd?: string }) => {
			seen.push(o?.cwd ?? "");
			return args[0] === "rev-parse" ? { stdout: "main\n", code: 0 } : { stdout: "", code: 0 };
		};
		expect(await level("bash", { command: "git push", cwd: "../other" }, exec)).toBe("deny");
		expect(seen[0]).toBe(`${HOME}/dev/other`);
	});
	test("command from foo.txt session asks", async () =>
		expect(await level("bash", { command: "rm foo.txt && ls foo.txt 2>&1 || true", cwd: `${HOME}/dev/random` })).toBe("ask"));
	test("session cwd inside protected dir denied", async () => expect(await level("read", { path: "x" }, fakeExec(), `${HOME}/.private`)).toBe("deny"));
	test("eval code naming protected dir denied", async () =>
		expect(await level("eval", { language: "py", code: "print(open('/home/u/.private/x').read())" })).toBe("deny"));
	test("hashline edit in protected dir denied", async () =>
		expect(await level("edit", { input: "*** Begin Patch\n[~/.private/x.md#ab12]\n+hi\n*** End Patch" })).toBe("deny"));
	test("without guards.json nothing extra is protected", async () =>
		expect((await evaluate("bash", { command: "ls ~/.private" }, CWD, fakeExec()))?.level ?? "allow").toBe("allow"));
});

describe("edit tool", () => {
	test("replace allowed", async () => expect(await level("edit", { path: "lib/x.ex", old_string: "a", new_string: "b" })).toBe("allow"));
	test("patch op delete asks", async () => expect(await level("edit", { path: "lib/x.ex", edits: [{ op: "delete" }] })).toBe("ask"));
	test("patch rename asks", async () => expect(await level("edit", { path: "lib/x.ex", edits: [{ rename: "lib/y.ex" }] })).toBe("ask"));
	test("apply_patch delete asks", async () =>
		expect(await level("edit", { input: "*** Begin Patch\n*** Delete File: lib/x.ex\n*** End Patch" })).toBe("ask"));
	test("hashline REM asks", async () => expect(await level("edit", { input: "*** Begin Patch\n[lib/x.ex#ab12]\nREM\n*** End Patch" })).toBe("ask"));
	test("hashline MV asks", async () => expect(await level("edit", { input: "*** Begin Patch\n[lib/x.ex#ab12]\nMV lib/y.ex\n*** End Patch" })).toBe("ask"));
	test("hashline edit of .env denied", async () => expect(await level("edit", { input: "*** Begin Patch\n[.env#ab12]\n+X=1\n*** End Patch" })).toBe("deny"));
	test("hashline normal edit allowed", async () =>
		expect(await level("edit", { input: "*** Begin Patch\n[lib/x.ex#ab12]\n+  def remove, do: :ok\n*** End Patch" })).toBe("allow"));
	test("patch body mentioning REM word allowed", async () =>
		expect(await level("edit", { input: "*** Begin Patch\n[lib/x.ex#ab12]\n+# REM is a unit\n*** End Patch" })).toBe("allow"));
});

describe("eval tool", () => {
	test("plain code allowed", async () => expect(await level("eval", { language: "py", code: "print(1+1)" })).toBe("allow"));
	test("os.environ asks", async () => expect(await level("eval", { language: "py", code: "import os; print(os.environ)" })).toBe("ask"));
	test("process.env asks", async () => expect(await level("eval", { language: "js", code: "console.log(process.env)" })).toBe("ask"));
	test("rmtree asks", async () => expect(await level("eval", { language: "py", code: "import shutil; shutil.rmtree('x')" })).toBe("ask"));
	test("subprocess asks", async () => expect(await level("eval", { language: "py", code: "import subprocess; subprocess.run(['ls'])" })).toBe("ask"));
	test("subprocess git add -A denied", async () =>
		expect(await level("eval", { language: "py", code: "import subprocess\nsubprocess.run('git add -A', shell=True)" })).toBe("deny"));
	test("open .env denied", async () => expect(await level("eval", { language: "py", code: "print(open('.env').read())" })).toBe("deny"));
});

describe("github tool", () => {
	test("read op allowed", async () => expect(await level("github", { op: "search_issues" })).toBe("allow"));
	test("pr_create asks", async () => expect(await level("github", { op: "pr_create", branch: "feature" })).toBe("ask"));
	test("pr_push to main denied", async () => expect(await level("github", { op: "pr_push", branch: "main" })).toBe("deny"));
});

describe("hook wiring", () => {
	type Handler = (event: unknown, ctx: unknown) => Promise<unknown>;
	function load() {
		let handler: Handler | undefined;
		const pi = { on: (name: string, h: Handler) => name === "tool_call" && (handler = h), exec: fakeExec() };
		guards(pi as never);
		return handler!;
	}
	const ctx = (hasUI: boolean, answer = false) => ({ hasUI, cwd: CWD, ui: { confirm: async () => answer } });

	test("deny blocks", async () =>
		expect(await load()({ toolName: "bash", input: { command: "git add -A" } }, ctx(true, true))).toMatchObject({ block: true }));
	test("ask without UI blocks", async () =>
		expect(await load()({ toolName: "bash", input: { command: "rm x" } }, ctx(false))).toMatchObject({ block: true }));
	test("ask with UI, user says no, blocks", async () =>
		expect(await load()({ toolName: "bash", input: { command: "rm x" } }, ctx(true, false))).toMatchObject({ block: true }));
	test("ask with UI, user says yes, runs", async () =>
		expect(await load()({ toolName: "bash", input: { command: "rm x" } }, ctx(true, true))).toBeUndefined());
	test("allow runs", async () => expect(await load()({ toolName: "bash", input: { command: "mix test" } }, ctx(false))).toBeUndefined());
});

describe("guards.json", () => {
	test("parses protectedDirs", () => expect(parseProtectedDirs('{"protectedDirs": ["~/.a", "/b", "", 3]}')).toEqual(["~/.a", "/b"]));
	test("no protectedDirs key", () => expect(parseProtectedDirs("{}")).toEqual([]));
	test("missing file means none", () => expect(loadProtectedDirs("/nonexistent/guards.json")).toEqual([]));
	test("trailing slash ignored", async () =>
		expect((await evaluate("bash", { command: "ls ~/.private" }, CWD, fakeExec(), [protectedDirRe("~/.private/", HOME)]))?.level).toBe("deny"));
});
