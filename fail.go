// Package fail simplifies error handling in tests using panic+recover
package fail

import (
	"reflect"
	"runtime/debug"
	"strings"
)

func Using(failer func(...interface{})) {
	r := recover()

	switch f := r.(type) {
	case nil:
		return
	case failure:
		parts := strings.Split(string(debug.Stack()), "\n")
		squashed := []string{""}
		found, include := false, false
		for _, part := range parts {
			if strings.Contains(part, pkgPath) {
				found = true
			} else if strings.HasPrefix(strings.TrimSpace(part), "testing.tRunner") {
				include, found = false, false
			} else if found {
				include = true
			}
			if include {
				squashed = append(squashed, part)
			}
		}
		failer(append(f, strings.Join(squashed, "\n"))...)
	default:
		panic(r)
	}
}

type failure []interface{}

type namer struct{}

var pkgPath = reflect.TypeOf(namer{}).PkgPath()

func IfErr(err error, args...interface{}) {
	if err != nil {
		panic(failure(append([]interface{}{err}, args...)))
	}
}

func IfDeferred(fn func() error, args...interface{}) {
	if err := fn(); err != nil {
		panic(failure(append([]interface{}{err}, args...)))
	}
}

func IfNot(condition bool, args ...interface{}) {
	if !condition {
		panic(failure(args))
	}
}
