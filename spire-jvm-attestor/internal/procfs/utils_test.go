package procfs

import (
	"testing"
)

func TestSanitizeSelector(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "colon stripped from javaagent flag",
			input: "-javaagent:",
			want:  "-javaagent_",
		},
		{
			name:  "colon stripped from jdwp flag",
			input: "-Xrunjdwp:",
			want:  "-Xrunjdwp_",
		},
		{
			name:  "equals sign replaced",
			input: "key=value",
			want:  "key_value",
		},
		{
			name:  "space replaced",
			input: "flag with space",
			want:  "flag_with_space",
		},
		{
			name:  "control characters replaced",
			input: "flag\x00\x01\x1f",
			want:  "flag___",
		},
		{
			name:  "clean flag unchanged",
			input: "-Xdebug",
			want:  "-Xdebug",
		},
		{
			name:  "clean flag with dots and hyphens unchanged",
			input: "-Dcom.sun.management.jmxremote",
			want:  "-Dcom.sun.management.jmxremote",
		},
		{
			name:  "multiple colons all replaced",
			input: "-agentlib:jdwp:transport=dt_socket",
			want:  "-agentlib_jdwp_transport_dt_socket",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeSelector(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeSelector(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeSelector_OutputIsValidSelectorValue(t *testing.T) {
	dangerous := []string{
		"-javaagent:/evil.jar",
		"-Xrunjdwp:transport=dt_socket,server=y",
		"-agentlib:hprof=heap=sites",
		"value with spaces",
		"tab\there",
		"\x00null\x00byte\x00",
	}
	for _, input := range dangerous {
		got := SanitizeSelector(input)
		for i, r := range got {
			if r == ':' || r == '=' || r == ' ' || r < 0x20 {
				t.Errorf(
					"SanitizeSelector(%q): illegal rune %q at position %d in output %q",
					input, r, i, got,
				)
			}
		}
	}
}

func TestParseEnviron(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "single entry",
			input: "PATH=/usr/bin\x00",
			want:  map[string]string{"PATH": "/usr/bin"},
		},
		{
			name:  "multiple entries",
			input: "PATH=/usr/bin\x00HOME=/root\x00USER=java\x00",
			want: map[string]string{
				"PATH": "/usr/bin",
				"HOME": "/root",
				"USER": "java",
			},
		},
		{
			name:  "value contains equals sign",
			input: "JAVA_TOOL_OPTIONS=-javaagent:/x.jar=opt\x00",
			want:  map[string]string{"JAVA_TOOL_OPTIONS": "-javaagent:/x.jar=opt"},
		},
		{
			name:  "empty value",
			input: "JAVA_TOOL_OPTIONS=\x00",
			want:  map[string]string{"JAVA_TOOL_OPTIONS": ""},
		},
		{
			name:  "key without equals stored with empty value",
			input: "BARE_KEY\x00",
			want:  map[string]string{"BARE_KEY": ""},
		},
		{
			name:  "trailing NUL skipped",
			input: "A=1\x00\x00\x00",
			want:  map[string]string{"A": "1"},
		},
		{
			name:  "empty input",
			input: "",
			want:  map[string]string{},
		},
		{
			name:  "no trailing NUL",
			input: "FOO=bar",
			want:  map[string]string{"FOO": "bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseEnviron(tt.input)

			if len(got) != len(tt.want) {
				t.Errorf("ParseEnviron(%q): got %d entries, want %d; got=%v want=%v",
					tt.input, len(got), len(tt.want), got, tt.want)
				return
			}
			for k, wantV := range tt.want {
				gotV, ok := got[k]
				if !ok {
					t.Errorf("ParseEnviron(%q): missing key %q", tt.input, k)
					continue
				}
				if gotV != wantV {
					t.Errorf("ParseEnviron(%q): key %q = %q, want %q", tt.input, k, gotV, wantV)
				}
			}
		})
	}
}

func TestParseEnviron_JavaToolOptionsScenarios(t *testing.T) {
	dangerous := map[string]string{
		"JAVA_TOOL_OPTIONS": "-javaagent:/evil.jar",
		"_JAVA_OPTIONS":     "-agentlib:jdwp",
		"JDK_JAVA_OPTIONS":  "-Xrunjdwp:transport=dt_socket",
	}

	raw := ""
	for k, v := range dangerous {
		raw += k + "=" + v + "\x00"
	}
	raw += "PATH=/usr/bin\x00"

	got := ParseEnviron(raw)

	for k, wantV := range dangerous {
		if got[k] != wantV {
			t.Errorf("key %q = %q, want %q", k, got[k], wantV)
		}
	}
	if got["PATH"] != "/usr/bin" {
		t.Errorf("PATH = %q, want /usr/bin", got["PATH"])
	}
}
