package fmts

func init() {
	const lang = "go"

	register(&Fmt{
		Name: "golint",
		Errorformat: []string{
			`%f:%l:%c: %m`,
		},
		Description: "linter for Go source code",
		URL:         "https://github.com/golang/lint",
		Language:    lang,
	})

	register(&Fmt{
		Name: "govet",
		Errorformat: []string{
			`%f:%l: %m`,
			`%-G%.%#`,
		},
		Description: "Vet examines Go source code and reports suspicious problems",
		URL:         "https://golang.org/cmd/vet/",
		Language:    lang,
	})
}
