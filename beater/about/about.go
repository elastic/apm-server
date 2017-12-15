package about

import "github.com/elastic/beats/libbeat/logp"

var about = make(map[string]interface{})

func SetAbout(version string) {
	logp.Info("APM Server Version %s  Git Ref %s Updated on %s", version, gitRef, updated)
	about["version"] = version
	about["git_ref"] = gitRef
	about["updated"] = updated
}

func About() map[string]interface{} {
	return about
}
