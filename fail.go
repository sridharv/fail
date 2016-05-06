// Package fail simplifies error handling using panic+recover
//
// It is primarily intended for use in tests. Sample usage is below:
//
// 	func TestSomething(t *testing.T) {
// 		// fail.Using recovers from panics and calls t.Fatal with a stack trace if any errors occur.
// 		defer fail.Using(t.Fatal)
//
// 		file, err := os.Open("myfile")
// 		// Panic with a failure if err is non-nil. The panic is recovered by the call to fail.Using
// 		// and t.Fatal is automatically called.
// 		fail.IfErr(err)
//
// 		defer fail.IfDeferred(file.Close, "failed to close myfile")
//
//   		data, err := ioutil.ReadAll(file)
// 		fail.IfErr(err, "failed to read contents of myfile")
// 		expected := "expected data"
// 		fail.If(string(data) != expected, string(data), " != ", expected)
// 	}
package fail

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
	"time"
)

func clean(str string) string {
	bracket, lineIndex := strings.LastIndex(str, "("), strings.LastIndex(str, " +0x")
	switch {
	case strings.HasSuffix(str, ")") && bracket != -1:
		return str[0:bracket]
	case lineIndex != -1:
		return str[0:lineIndex]
	default:
		return str
	}
}

func squash(trace []string) ([]string, bool) {
	squashed := []string{""}
	found, include, strippedPath := false, false, false
	for _, part := range trace {
		if strings.Contains(part, pkgPath) {
			found, strippedPath = true, true
		} else if strings.HasPrefix(strings.TrimSpace(part), "testing.tRunner") {
			include, found = false, false
		} else if found {
			include = true
		}
		if include {
			squashed = append(squashed, clean(part))
		}
	}
	if !strippedPath {
		return trace, false
	}
	return squashed, true
}

// Using recovers from panics and calls failure with the result of the recovery. It must be used as part of a deferred
// call.
func Using(failer func(...interface{})) {
	r := recover()

	switch f := r.(type) {
	case nil:
		return
	case failure:
		trace := strings.Split(string(debug.Stack()), "\n")
		squashed, more := squash(trace)
		for i := 0; i < 3 && more; i++ {
			squashed, more = squash(squashed)
		}
		res := append(f, strings.Join(squashed, "\n"))
		res = append(res, queue...)
		failing, queue = false, []interface{}{}
		failer(res...)
	default:
		panic(r)
	}
}

type failure []interface{}

type namer struct{}

var pkgPath = reflect.TypeOf(namer{}).PkgPath()

// TimedOut returns true if the function passed in takes longer than
// timeout to run.
func TimedOut(fn func(), timeout time.Duration) bool {
	ch := make(chan struct{})
	go func() {
		fn()
		close(ch)
	}()
	select {
	case <-ch:
		return false
	case <-time.After(timeout):
		return true
	}
}

// IfErr panics if err is non-nil, constructing a failure message with the error and args.
// It must be used in conjunction with Using.
func IfErr(err error, args ...interface{}) {
	if err != nil {
		Now(append([]interface{}{err}, args...)...)
	}
}

// IfDeferred panics if the error returned by fn is non-nil, constructing a failure message with the error and args.
// It must be used in conjunction with Using, to check for errors in deferred functions. Sample usage is below:
//
// 	func TestSomething(t *testing.T) {
// 		defer fail.Using(t.Fatal)
// 		file, err := os.Open("myfile")
//  		fail.IfErr(err)
// 		defer fail.IfDeferred(file.Close, "closing myfile failed")
// 		...
// 	}
func IfDeferred(fn func() error, args ...interface{}) {
	if err := fn(); err != nil {
		Now(append([]interface{}{err}, args...)...)
	}
}

// If panics if the condition is true, constructing a failure message with the arguments passed in.
// It must be used in conjuction with Using.
func If(condition bool, args ...interface{}) {
	if condition {
		Now(args...)
	}
}

var failing = false
var queue = []interface{}{}

func enqueue(f interface{}) {
	defer func() {
		r := recover().(failure)
		squashed := []string{"", "Failure on defer: " + fmt.Sprintln(r...)}
		queue = append(queue, strings.Join(squashed, "\n"))
	}()
	panic(f)
}

// Message returns a failure message that can be recovered by a call to Using.
func Message(args ...interface{}) interface{} {
	failing = true
	return failure(args)
}

// Now is equivalent to panic(Message(...)). However, if Now is being called
// as part of a deferred statement and another failure as already occurred, the
// failure will be added to the original failure.
func Now(args ...interface{}) {
	if !failing {
		panic(Message(args...))
	}
	enqueue(Message(args...))
}
