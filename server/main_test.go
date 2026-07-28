package main

import (
	"reflect"
	"testing"
)

func TestParseCorsOrigins(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			// The bug: a browser's Origin header has no leading space, so the
			// second entry could never match and that site was silently blocked.
			name: "trims whitespace after the separator",
			raw:  "https://a.example.com, https://b.example.com",
			want: []string{"https://a.example.com", "https://b.example.com"},
		},
		{
			name: "leaves a well-formed list alone",
			raw:  "https://a.example.com,https://b.example.com",
			want: []string{"https://a.example.com", "https://b.example.com"},
		},
		{
			name: "handles surrounding and tab whitespace",
			raw:  "  https://a.example.com\t,\nhttps://b.example.com  ",
			want: []string{"https://a.example.com", "https://b.example.com"},
		},
		{
			name: "drops empty entries from a trailing comma",
			raw:  "https://a.example.com,",
			want: []string{"https://a.example.com"},
		},
		{
			// Falls through to the caller's default list rather than
			// configuring an origin that matches nothing.
			name: "unset yields no origins",
			raw:  "",
			want: []string{},
		},
		{
			name: "whitespace-only yields no origins",
			raw:  " , ",
			want: []string{},
		},
		{
			name: "single origin",
			raw:  "http://localhost:8080",
			want: []string{"http://localhost:8080"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCorsOrigins(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseCorsOrigins(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}
