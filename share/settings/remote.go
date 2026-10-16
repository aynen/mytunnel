package settings

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// short-hand conversions (see remote_test)
//   3000 ->
//     local  127.0.0.1:3000
//     remote 127.0.0.1:3000
//   foobar.com:3000 ->
//     local  127.0.0.1:3000
//     remote foobar.com:3000
//   3000:google.com:80 ->
//     local  127.0.0.1:3000
//     remote google.com:80
//   192.168.0.1:3000:google.com:80 ->
//     local  192.168.0.1:3000
//     remote google.com:80

type Remote struct {
	LocalHost, LocalPort string
	RemoteHost, RemotePort string
}

func DecodeRemote(s string) (*Remote, error) {
	parts := regexp.MustCompile(`(\[[^\[\]]+\]|[^\[\]:]+):?`).FindAllStringSubmatch(s, -1)
	if len(parts) <= 0 || len(parts) >= 5 {
		return nil, errors.New("Invalid remote")
	}
	r := &Remote{}
	//parse from back to front, to set 'remote' fields first,
	//then to set 'local' fields second (allows the 'remote' side
	//to provide the defaults)
	for i := len(parts) - 1; i >= 0; i-- {
		p := parts[i][1]
		if isPort(p) {
			if r.RemotePort == "" {
				r.RemotePort = p
			}
			r.LocalPort = p
			continue
		}
		if r.RemotePort == "" && r.LocalPort == "" {
			return nil, errors.New("Missing ports")
		}
		if !isHost(p) {
			return nil, errors.New("Invalid host")
		}
		if r.RemoteHost == "" {
			r.RemoteHost = p
		} else {
			r.LocalHost = p
		}
	}
	//remote string parsed, apply defaults...
	//non-socks defaults
	if r.LocalHost == "" {
		r.LocalHost = "0.0.0.0"
	}
	if r.RemoteHost == "" {
		r.RemoteHost = "127.0.0.1"
	}
	//validate port numbers
	if r.LocalPort != "" {
		if !isPort(r.LocalPort) {
			return nil, errors.New("Invalid local port")
		}
	}
	if r.RemotePort != "" {
		if !isPort(r.RemotePort) {
			return nil, errors.New("Invalid remote port")
		}
	}
	return r, nil
}

func isPort(s string) bool {
	n, err := strconv.Atoi(s)
	if err != nil {
		return false
	}
	if n <= 0 || n > 65535 {
		return false
	}
	return true
}

func isHost(s string) bool {
	_, err := url.Parse("//" + s)
	if err != nil {
		return false
	}
	return true
}

//implement Stringer
func (r Remote) String() string {
	sb := strings.Builder{}
	sb.WriteString(strings.TrimPrefix(r.Local(), "0.0.0.0:"))
	sb.WriteString("=>")
	sb.WriteString(strings.TrimPrefix(r.Remote(), "127.0.0.1:"))
	return sb.String()
}

//Encode remote to a string
func (r Remote) Encode() string {
	if r.LocalPort == "" {
		r.LocalPort = r.RemotePort
	}
	local := r.Local()
	remote := r.Remote()
	return local + ":" + remote
}

//Local is the decodable local portion
func (r Remote) Local() string {
	if r.LocalHost == "" {
		r.LocalHost = "0.0.0.0"
	}
	return r.LocalHost + ":" + r.LocalPort
}

//Remote is the decodable remote portion
func (r Remote) Remote() string {
	if r.RemoteHost == "" {
		r.RemoteHost = "127.0.0.1"
	}
	return r.RemoteHost + ":" + r.RemotePort
}

//UserAddr is checked when checking if a
//user has access to a given remote
func (r Remote) UserAddr() string {
	return r.RemoteHost + ":" + r.RemotePort
}

//CanListen checks if the port can be listened on
func (r Remote) CanListen() bool {
	conn, err := net.Listen("tcp", r.Local())
	if err == nil {
		conn.Close()
		return true
	}
	return false
}

type Remotes []*Remote

//Encode back into strings
func (rs Remotes) Encode() []string {
	s := make([]string, len(rs))
	for i, r := range rs {
		s[i] = r.Encode()
	}
	return s
}