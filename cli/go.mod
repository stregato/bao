module github.com/stregato/bao/cli

go 1.23.0

require (
	github.com/stregato/bao/lib v0.0.0
	golang.org/x/term v0.15.0
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/stregato/bao/lib => ../lib
