package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var doctorFix bool

type issue struct {
	Check  string `json:"check"`
	ID     string `json:"id,omitempty"`
	Ref    string `json:"ref,omitempty"`
	Detail string `json:"detail"`
	Fixed  bool   `json:"fixed,omitempty"`
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check data integrity and report problems",
	Long: `Checks for orphaned docket entries, dangling relationship references,
and bidirectional link inconsistencies. Use --fix to repair automatically.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var issues []issue

		matterIDs := make(map[string]bool)
		all, err := store.ListMatters(nil)
		if err != nil {
			return err
		}
		for _, m := range all {
			matterIDs[m.ID] = true
		}

		// Check 1: Orphaned docket entries
		entries, err := store.LoadDocket()
		if err != nil {
			return err
		}
		for _, e := range entries {
			if !matterIDs[e.ID] {
				iss := issue{Check: "orphaned-docket", ID: e.ID, Detail: "docket references nonexistent matter"}
				if doctorFix {
					if fixErr := store.DocketRemove(e.ID); fixErr == nil {
						iss.Fixed = true
					}
				}
				issues = append(issues, iss)
			}
		}

		// Check 2: Dangling relationship references
		for _, m := range all {
			for _, ref := range m.Relates {
				if !matterIDs[ref] {
					iss := issue{Check: "dangling-relates", ID: m.ID, Ref: ref, Detail: fmt.Sprintf("%s relates to nonexistent %s", m.ID, ref)}
					if doctorFix {
						m.Relates = removeFromSlice(m.Relates, ref)
						iss.Fixed = true
					}
					issues = append(issues, iss)
				}
			}
			for _, ref := range m.Blocks {
				if !matterIDs[ref] {
					iss := issue{Check: "dangling-blocks", ID: m.ID, Ref: ref, Detail: fmt.Sprintf("%s blocks nonexistent %s", m.ID, ref)}
					if doctorFix {
						m.Blocks = removeFromSlice(m.Blocks, ref)
						iss.Fixed = true
					}
					issues = append(issues, iss)
				}
			}
			for _, ref := range m.Needs {
				if !matterIDs[ref] {
					iss := issue{Check: "dangling-needs", ID: m.ID, Ref: ref, Detail: fmt.Sprintf("%s needs nonexistent %s", m.ID, ref)}
					if doctorFix {
						m.Needs = removeFromSlice(m.Needs, ref)
						iss.Fixed = true
					}
					issues = append(issues, iss)
				}
			}
			if m.Parent != "" && !matterIDs[m.Parent] {
				iss := issue{Check: "dangling-parent", ID: m.ID, Ref: m.Parent, Detail: fmt.Sprintf("%s has nonexistent parent %s", m.ID, m.Parent)}
				if doctorFix {
					m.Parent = ""
					iss.Fixed = true
				}
				issues = append(issues, iss)
			}

			// Write back if we fixed references on this matter
			if doctorFix {
				for i := range issues {
					if issues[i].ID == m.ID && issues[i].Fixed {
						if err := store.WriteMatter(m); err != nil {
							return fmt.Errorf("fixing %s: %w", m.ID, err)
						}
						break
					}
				}
			}
		}

		// Check 3: Bidirectional link consistency
		for _, m := range all {
			for _, ref := range m.Blocks {
				if !matterIDs[ref] {
					continue // already caught above
				}
				other, err := store.GetMatter(ref)
				if err != nil {
					continue
				}
				if !sliceContains(other.Needs, m.ID) {
					iss := issue{Check: "asymmetric-blocks", ID: m.ID, Ref: ref, Detail: fmt.Sprintf("%s blocks %s but %s doesn't need %s", m.ID, ref, ref, m.ID)}
					if doctorFix {
						if fixErr := store.LinkMatters(m.ID, "blocks", ref); fixErr == nil {
							iss.Fixed = true
						}
					}
					issues = append(issues, iss)
				}
			}
			for _, ref := range m.Needs {
				if !matterIDs[ref] {
					continue
				}
				other, err := store.GetMatter(ref)
				if err != nil {
					continue
				}
				if !sliceContains(other.Blocks, m.ID) {
					iss := issue{Check: "asymmetric-needs", ID: m.ID, Ref: ref, Detail: fmt.Sprintf("%s needs %s but %s doesn't block %s", m.ID, ref, ref, m.ID)}
					if doctorFix {
						if fixErr := store.LinkMatters(ref, "blocks", m.ID); fixErr == nil {
							iss.Fixed = true
						}
					}
					issues = append(issues, iss)
				}
			}
			for _, ref := range m.Relates {
				if !matterIDs[ref] {
					continue
				}
				other, err := store.GetMatter(ref)
				if err != nil {
					continue
				}
				if !sliceContains(other.Relates, m.ID) {
					iss := issue{Check: "asymmetric-relates", ID: m.ID, Ref: ref, Detail: fmt.Sprintf("%s relates to %s but not vice versa", m.ID, ref)}
					if doctorFix {
						if fixErr := store.LinkMatters(m.ID, "relates", ref); fixErr == nil {
							iss.Fixed = true
						}
					}
					issues = append(issues, iss)
				}
			}
		}

		// Check 4: Done/dropped matters still in docket
		// Re-load docket in case fix mode already removed orphans
		entries, err = store.LoadDocket()
		if err != nil {
			return err
		}
		for _, e := range entries {
			m, err := store.GetMatter(e.ID)
			if err != nil {
				continue // orphan already handled
			}
			if m.IsTerminal() {
				iss := issue{Check: "docket-terminal", ID: e.ID, Detail: fmt.Sprintf("%s matter %s is still in docket", m.Status, e.ID)}
				if doctorFix {
					if fixErr := store.DocketRemove(e.ID); fixErr == nil {
						iss.Fixed = true
					}
				}
				issues = append(issues, iss)
			}
		}

		if issues == nil {
			issues = []issue{}
		}

		out := map[string]any{"issues": issues, "count": len(issues)}
		if doctorFix {
			fixed := 0
			for _, iss := range issues {
				if iss.Fixed {
					fixed++
				}
			}
			out["fixed"] = fixed
		}

		return json.NewEncoder(os.Stdout).Encode(out)
	},
}

func removeFromSlice(slice []string, s string) []string {
	var result []string
	for _, v := range slice {
		if v != s {
			result = append(result, v)
		}
	}
	return result
}

func sliceContains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Automatically fix issues")
	rootCmd.AddCommand(doctorCmd)
}
