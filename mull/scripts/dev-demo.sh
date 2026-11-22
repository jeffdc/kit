#!/bin/sh
# Creates the demo repository make dev serves, if it isn't there: a scratch
# git repo on mull with something in every Needs you tier, for trying the UI
# by hand or from a headless browser. Usage: dev-demo.sh <dev dir> <mull>.
# Everything lives under <dev dir>; its config dir keeps the registry and
# last-looked state away from the real ones.
set -eu
dev=$1 mull=$2
demo=$dev/demo
[ -d "$demo/.git" ] && exit 0

mkdir -p "$demo" "$dev/cfg"
export XDG_CONFIG_HOME="$dev/cfg"
cd "$demo"
git init -q -b main
git commit -q --allow-empty -m "demo"
m() { "$mull" "$@" >/dev/null; }
id() { "$mull" "$@" | sed -n 's/^  "id": "\([0-9a-f]*\)".*/\1/p' | head -1; }

m init
echo '{"op": "config checks", "commands": ["true"], "on-request": true}' | "$mull" batch >/dev/null
m plan new web "Web app" --summary "Search, mail and the account pages." --on-request
m plan agree web --on-request
m plan doc web - <<'EOF'
The web app's search, mail and account pages. This doc is long enough to show how prose runs on the sheet, with the status margin beside it: search comes first because mail links into it.
EOF
# A phase with a doc, for the phase sheet.
launch=$(id phase add "Launch" --plan web --summary "Turn the pages on for everyone.")
m phase doc "$launch" - <<'EOF'
Turn the flag on once search and mail are in. Done when the pages are live for everyone.
EOF

# A ready item with a question on it: the Blocking tier.
mail=$(id add "Mailer" --plan web)
m ask "Which queue do mails go through?" --item "$mail"
# Decisions from an item, and from a planning session: the Decisions tier.
search=$(id add "Search page" --plan web)
m decided "Search runs on Postgres full-text, not a separate index" --item "$search"
m decided "Results page 25 per page" --item "$search"
m decided "Keep the API on /v2" --plan web --source "planning call"
# An item in review carrying decisions: the Review tier and a packet.
digest=$(id add "Digest job" --plan web)
m start "$digest"
m decided "Digests skip users with no activity" --item "$digest"
m decided "One digest per timezone batch" --item "$digest"
m done "$digest" --summary "A nightly digest job." --tour none --design "A cron job reading the activity table."
# A long title, and a captured bug for the backlog.
m add "Drop or park a Needs you entry with a reason" --plan web
m bug "The footer overlaps the last result on small screens" --plan web --source "demo"
# Unplanned work: one open, and two finished (done, and dropped), which are
# archived at once, under the sheet index's Archive.
m add "Fix the login redirect" --plan none
fav=$(id add "Add a favicon" --plan none)
m start "$fav"
m done "$fav" --summary "A favicon." --tour none --design "An SVG in the page head."
m accept "$fav" --on-request
old=$(id add "Rename the settings page" --plan none)
m drop "$old" --reason "Not needed after all" --on-request
# The web plan lands on its own target branch, a commit ahead of main, with
# the digest job's work merged into it.
git checkout -q -b web-next
git checkout -q -b "$digest-job"
git commit -q --allow-empty -m "Nightly digest job" -m "Mull: $digest"
git checkout -q web-next
git merge -q --no-ff --no-edit "$digest-job"
git checkout -q main
m plan set web target web-next
# A phase that needs an integration review, its one item done and merged:
# it awaits that review, and Launch, which needs it, waits.
pass=$(id phase add "Design pass" --plan web --summary "One look across every page.")
m phase set "$pass" integration-review on --on-request
header=$(id add "Shared page header" --phase "$pass")
m start "$header"
git checkout -q web-next
git commit -q --allow-empty -m "Shared page header" -m "Mull: $header"
git checkout -q main
m done "$header" --summary "One header for every page." --tour none --design "A shared partial."
m accept "$header" --on-request
m need "$launch" "$pass"
# Two agents at work on branches that both change the page layout: an
# overlap in the Plans column.
account=$(id add "Account page" --plan web)
theme=$(id add "Dark theme" --plan web)
for it in "$account:account" "$theme:theme"; do
	item=${it%%:*} name=${it#*:}
	git checkout -q -b "$item-$name" web-next
	m start "$item"
	mkdir -p app
	echo "$name" >"app/$name.css"
	echo "/* $name */" >app/layout.css
	git add app
	git commit -q -m "Start the $name work" -m "Mull: $item"
done
git checkout -q main
# A plan still shaping, for Agree this plan.
m plan new pilot "Pilot with two teams" --summary "Try the web app with two teams before launch." --on-request
echo "demo repo: $demo"
