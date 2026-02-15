package cmd

import "mull/internal/model"

// excludeTerminal filters out done and dropped matters.
func excludeTerminal(matters []*model.Matter) []*model.Matter {
	filtered := make([]*model.Matter, 0, len(matters))
	for _, m := range matters {
		if m.Status != "done" && m.Status != "dropped" {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// excludeDocketed filters out matters that appear on the docket.
func excludeDocketed(matters []*model.Matter) ([]*model.Matter, error) {
	entries, err := store.LoadDocket()
	if err != nil {
		return nil, err
	}
	onDocket := make(map[string]bool, len(entries))
	for _, e := range entries {
		onDocket[e.ID] = true
	}
	filtered := make([]*model.Matter, 0, len(matters))
	for _, m := range matters {
		if !onDocket[m.ID] {
			filtered = append(filtered, m)
		}
	}
	return filtered, nil
}
