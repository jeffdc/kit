package serve

import (
	"strconv"

	"mull/internal/mull"
)

// resolve returns the path of the registered project with this ID.
func resolve(repos Repos, id string) (string, error) {
	paths, err := repos()
	if err != nil {
		return "", err
	}
	for i, pid := range projectIDs(paths) {
		if pid == id {
			return paths[i], nil
		}
	}
	return "", &mull.Error{Code: mull.CodeNotFound, Msg: "no project " + strconv.Quote(id)}
}
