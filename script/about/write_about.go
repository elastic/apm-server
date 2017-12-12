package main

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"time"
)

func main() {

	p := exec.Command("git", "rev-parse", "HEAD")
	out, err := p.Output()
	if err != nil {
		panic(err)
	}

	ref := strings.TrimSpace(string(out))
	source := fmt.Sprintf(`package about

const gitRef = "%s"
const updated = "%s"
`, ref, time.Now().UTC().Truncate(time.Second))

	err = ioutil.WriteFile("./beater/about/generated.go", []byte(source), 0644)

	if err != nil {
		panic(err)
	}
}
