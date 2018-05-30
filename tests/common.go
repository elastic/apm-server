package tests

func strConcat(pre string, post string, delimiter string) string {
	if pre == "" {
		return post
	}
	return pre + delimiter + post
}
