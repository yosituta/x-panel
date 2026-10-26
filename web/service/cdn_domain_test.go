package service

import "testing"

func TestTrailingDigits(t *testing.T) {
	cases := map[string]string{
		"panel2": "2",
		"us1":    "1",
		"cdn":    "",
		"hk12":   "12",
		"":       "",
	}
	for in, want := range cases {
		if got := trailingDigits(in); got != want {
			t.Errorf("trailingDigits(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBaseDomain(t *testing.T) {
	cases := map[string]string{
		"panel1.example.com": "example.com",
		"example.com":        "com",
		"":                   "",
	}
	for in, want := range cases {
		if got := baseDomain(in); got != want {
			t.Errorf("baseDomain(%q) = %q, want %q", in, got, want)
		}
	}
}
