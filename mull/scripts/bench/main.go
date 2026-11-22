// Command bench seeds large mull repositories and times every page mull
// serve shows. It's a development tool, not part of mull; make bench runs it.
//
//	go run ./scripts/bench seed -size large -dir .dev/bench/large
//	go run ./scripts/bench run [-n 5] [-url http://127.0.0.1:6855] [-json] <repo>...
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"mull/internal/bench"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "seed":
		err = seed(os.Args[2:])
	case "run":
		err = run(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: bench seed -size small|medium|large -dir <dir> [-seed n] [-targets n]\n       bench run [-n runs] [-url live-server] [-back 10,1000,10000] [-json] <repo>...")
	os.Exit(2)
}

func seed(args []string) error {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	size := fs.String("size", "large", "small, medium or large")
	dir := fs.String("dir", "", "where to create the repository (skipped if it exists)")
	n := fs.Uint64("seed", 1, "random seed: the same seed gives the same records")
	targets := fs.Int("targets", 0, "how many active plans (0 to 2) get a target branch of their own")
	fs.Parse(args)
	s, ok := bench.Sizes[*size]
	if !ok || *dir == "" {
		usage()
	}
	s.Targets = *targets
	if _, err := os.Stat(*dir); err == nil {
		fmt.Fprintf(os.Stderr, "%s exists; not reseeding\n", *dir)
		return nil
	}
	t := time.Now()
	if err := bench.Seed(*dir, s, *n, time.Now()); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "seeded %s (%s) in %v\n", *dir, *size, time.Since(t).Round(time.Millisecond))
	return nil
}

func run(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	runs := fs.Int("n", 5, "timed runs per target, after one cold run")
	live := fs.String("url", "", "also time this running mull serve (e.g. the installed one)")
	back := fs.String("back", "10,1000,10000", "revision distances, in mull commits")
	asJSON := fs.Bool("json", false, "print JSON")
	fs.Parse(args)
	if fs.NArg() == 0 {
		usage()
	}
	var dists []int
	for _, f := range strings.Split(*back, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil {
			return fmt.Errorf("-back: %w", err)
		}
		dists = append(dists, n)
	}
	cfg, err := bench.Isolate()
	if err != nil {
		return err
	}
	defer os.RemoveAll(cfg)
	rep, err := bench.Run(fs.Args(), bench.Options{Runs: *runs, URL: *live, Back: dists})
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	printText(rep, *live != "")
	return nil
}

// row is one target's results, by how it was reached.
type row struct {
	in, http, live *bench.Result
}

// printText prints one table per repository: a row per target, with the
// median in-process and over HTTP, the cold first call, and git calls.
func printText(rep *bench.Report, live bool) {
	for _, sh := range rep.Shapes {
		name := sh.Repo[strings.LastIndex(sh.Repo, "/")+1:]
		fmt.Printf("\n%s  (%d files, %d commits; %d plans, %d phases, %d items of which %d done or dropped, %d inbox entries)\n",
			name, sh.Files, sh.Commits, sh.Plans, sh.Phases, sh.Items, sh.Done, sh.Entries)
		var order []string
		rows := map[string]*row{}
		for i := range rep.Results {
			r := &rep.Results[i]
			if r.Repo != name {
				continue
			}
			if rows[r.Target] == nil {
				rows[r.Target] = &row{}
				order = append(order, r.Target)
			}
			switch r.Over {
			case "in-process":
				rows[r.Target].in = r
			case "http":
				rows[r.Target].http = r
			case "live":
				rows[r.Target].live = r
			}
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
		head := "target\tin-process\tgit\thttp\thttp cold\tbytes\t"
		if live {
			head += "live\tlive cold\t"
		}
		fmt.Fprintln(w, head)
		for _, t := range order {
			r := rows[t]
			line := t + "\t" + med(r.in) + "\t" + gitCalls(r) + "\t" + med(r.http) + "\t" + cold(r.http) + "\t" + size(r.http) + "\t"
			if live {
				line += med(r.live) + "\t" + cold(r.live) + "\t"
			}
			fmt.Fprintln(w, line)
		}
		w.Flush()
	}
	fmt.Println("\nTimes are medians in ms (cold: the first call). git: git processes one call starts.")
}

func med(r *bench.Result) string {
	if r == nil {
		return "-"
	}
	return fmt.Sprintf("%.1f", r.MedMS)
}

func cold(r *bench.Result) string {
	if r == nil {
		return "-"
	}
	return fmt.Sprintf("%.1f", r.ColdMS)
}

func size(r *bench.Result) string {
	if r == nil {
		return "-"
	}
	return strconv.Itoa(r.Bytes)
}

func gitCalls(r *row) string {
	switch {
	case r.in != nil:
		return strconv.FormatInt(r.in.Git, 10)
	case r.http != nil:
		return strconv.FormatInt(r.http.Git, 10)
	}
	return "-"
}
