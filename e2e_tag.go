//go:build e2e

package main

// The automation socket that the end-to-end test drives. It is compiled only
// with -tags e2e and never into a release build.
import _ "github.com/egoist/mygo/plugins/automation"
