package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "no args", args: nil, wantCode: 2, wantStderr: "Usage:"},
		{name: "help", args: []string{"help"}, wantCode: 0, wantStdout: "Usage:"},
		{name: "help flag", args: []string{"--help"}, wantCode: 0, wantStdout: "Usage:"},
		{name: "unknown command", args: []string{"nope"}, wantCode: 2, wantStderr: `unknown command "nope"`},
		{name: "old push command is gone", args: []string{"push"}, wantCode: 2, wantStderr: `unknown command "push"`},
		{name: "db without subcommand", args: []string{"db"}, wantCode: 2, wantStderr: "prismo db <command>"},
		{name: "db help", args: []string{"db", "help"}, wantCode: 0, wantStdout: "prismo db <command>"},
		{name: "unknown db command", args: []string{"db", "pull"}, wantCode: 2, wantStderr: `unknown db command "pull"`},
		{name: "db push help", args: []string{"db", "push", "-h"}, wantCode: 0, wantStderr: "-schema"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), tt.args, &stdout, &stderr)

			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
