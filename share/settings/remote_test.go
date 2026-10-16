package settings

import (
	"reflect"
	"testing"
)

func TestRemoteDecode(t *testing.T) {
	//test table
	for i, test := range []struct {
		Input   string
		Output  Remote
		Encoded string
	}{
		{
			"3000",
			Remote{
				LocalPort:  "3000",
				RemoteHost: "127.0.0.1",
				RemotePort: "3000",
			},
			"0.0.0.0:3000:127.0.0.1:3000",
		},
		{
			"google.com:80",
			Remote{
				LocalPort:  "80",
				RemoteHost: "google.com",
				RemotePort: "80",
			},
			"0.0.0.0:80:google.com:80",
		},
		{
			"示例網站.com:80",
			Remote{
				LocalPort:  "80",
				RemoteHost: "示例網站.com",
				RemotePort: "80",
			},
			"0.0.0.0:80:示例網站.com:80",
		},
		{
			"[::1]:8080:google.com:80",
			Remote{
				LocalHost:  "[::1]",
				LocalPort:  "8080",
				RemoteHost: "google.com",
				RemotePort: "80",
			},
			"[::1]:8080:google.com:80",
		},
	} {
		//expected defaults
		expected := test.Output
		if expected.LocalHost == "" {
			expected.LocalHost = "0.0.0.0"
		}
		//compare
		got, err := DecodeRemote(test.Input)
		if err != nil {
			t.Fatalf("decode #%d '%s' failed: %s", i+1, test.Input, err)
		}
		if !reflect.DeepEqual(got, &expected) {
			t.Fatalf("decode #%d '%s' expected\n  %#v\ngot\n  %#v", i+1, test.Input, expected, got)
		}
		if e := got.Encode(); test.Encoded != e {
			t.Fatalf("encode #%d '%s' expected\n  %#v\ngot\n  %#v", i+1, test.Input, test.Encoded, e)
		}
	}
}

func TestRemoteDecodeErrors(t *testing.T) {
	for _, input := range []string{
		"",
		"foo",
		"foo:bar:baz:qux:quux",
		"foo:bar",           // no port
		"80:65536",          // invalid remote port > 65535
		"localhost:99999",   // invalid local port > 65535
	} {
		_, err := DecodeRemote(input)
		if err == nil {
			t.Errorf("expected error for input '%s'", input)
		}
	}
}