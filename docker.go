package umpire

var configMap = map[string]struct {
	Cmd   []string
	Image string
}{
	"cpp": {[]string{"-stream=true"}, "phluent/clang"},
}
