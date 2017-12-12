package version

import "fmt"

var (
	buildTime = "unknown"
	commit    = "unknown"
)

func String() string {
	return fmt.Sprintf("%s built %s", commit, buildTime)
}
