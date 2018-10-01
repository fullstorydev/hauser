// Copyright 2017 FullStory, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gorepoman

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

func ErrWithMessagef(err error, format string, a ...interface{}) error {
	return errors.WithMessage(err, fmt.Sprintf(format, a...))
}

func PrintError(w io.Writer, err error) {
	// Messages come back from deepest to shallowest; reverse the order for printing.
	msgs, stack := deconstruct(err)
	for i := range msgs {
		if i > 0 {
			fmt.Fprint(w, "...caused by ")
		}
		msg := msgs[len(msgs)-i-1]
		if msg[len(msg)-1] != '\n' {
			msg = msg + "\n"
		}
		fmt.Fprint(w,msg)
	}
	if stack != nil {
		fmt.Fprintf(w, "%+v\n", stack)
	}
}

func deconstruct(err error) ([]string, errors.StackTrace) {
	if err == nil {
		return nil, nil
	}

	myMsg := err.Error()
	var messages []string
	var stack errors.StackTrace

	type causer interface {
		Cause() error
	}
	if causer, ok := err.(causer); ok {
		messages, stack = deconstruct(causer.Cause())
		if len(messages) > 0 {
			pos := strings.Index(myMsg, ": "+messages[len(messages)-1])
			if pos >= 0 {
				myMsg = myMsg[:pos]
			}
		}
	}

	messages = append(messages, myMsg)

	if stack == nil {
		type stackTracer interface {
			StackTrace() errors.StackTrace
		}
		if tracer, ok := err.(stackTracer); ok {
			stack = tracer.StackTrace()
		}
	}

	return messages, stack
}

// Helper to check if a particular folder exists
func Exists(path string) bool {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	} else if err != nil {
		panic(err)
	}
	return true
}

// Wrapper function to copy one directory to another
// Implementing this in go would have been more trouble than it was worth, better to just call cp -R
func copyDir(src string, dest string) error {
	_, err := execBinary("cp", nil, "-R", src, dest)
	if err != nil {
		return ErrWithMessagef(err, "Error copying directory %s to %s", src, dest)
	}

	return nil
}

// Load environment variables and fail if they do not exist
func assertEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		fmt.Fprintln(os.Stderr, name+" environment variable not defined. Required for operation.")
		os.Exit(1)
	}
	return v
}

// execBinary runs a given binary by looking up its path and executing it with given params env vars
// On success, returns the combined output (stdout + stderr).  On failure (non-zero exit status),
// the command output is returned as an error.
func execBinary(binary string, envVars []string, params ...string) (string, error) {
	cmd := exec.Command(binary, params...)
	// Grab current environment variables
	env := os.Environ()
	cmd.Env = combineEnv(env, envVars)

	cmdOut, err := cmd.CombinedOutput()
	out := string(cmdOut)
	if err != nil {
		if out != "" {
			// Use the command output as the error; err is probably just an exit code.
			return "", errors.New(strings.TrimSpace(out))
		} else {
			return "", errors.WithStack(err)
		}
	}
	return out, nil
}

func combineEnv(orig, override []string) []string {
	env := map[string]string{}
	for _, e := range orig {
		parts := strings.SplitN(e, "=", 2)
		env[parts[0]] = parts[1]
	}
	for _, e := range override {
		parts := strings.SplitN(e, "=", 2)
		env[parts[0]] = parts[1]
	}
	ret := make([]string, len(env))
	i := 0
	for k, v := range env {
		ret[i] = fmt.Sprintf("%s=%s", k, v)
		i++
	}
	sort.Strings(ret)
	return ret
}
