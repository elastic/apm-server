package javaattacher

import "regexp"

type DiscoveryRule interface {
	include() bool
	match(jvm *JvmDetails) bool
}

type IncludeAllRule struct{}

func (IncludeAllRule) match(jvm *JvmDetails) bool {
	return true
}

func (IncludeAllRule) include() bool {
	return true
}

type UserDiscoveryRule struct {
	isIncludeRule bool
	user          string
}

func (rule UserDiscoveryRule) match(jvm *JvmDetails) bool {
	return jvm.user == rule.user
}

func (rule UserDiscoveryRule) include() bool {
	return rule.isIncludeRule
}

type CmdLineDiscoveryRule struct {
	isIncludeRule bool
	regex         *regexp.Regexp
}

func (rule CmdLineDiscoveryRule) match(jvm *JvmDetails) bool {
	return rule.regex.MatchString(jvm.cmdLineArgs)
}

func (rule CmdLineDiscoveryRule) include() bool {
	return rule.isIncludeRule
}
