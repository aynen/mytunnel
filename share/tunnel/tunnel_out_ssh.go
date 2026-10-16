package tunnel

import (
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/jpillora/chisel/share/cio"
	"github.com/jpillora/sizestr"
	"golang.org/x/crypto/ssh"
)

func (t *Tunnel) handleSSHRequests(reqs <-chan *ssh.Request) {
	for r := range reqs {
		switch r.Type {
		case "ping":
			r.Reply(true, []byte("pong"))
		default:
			t.Debugf("Unknown request: %s", r.Type)
		}
	}
}

func (t *Tunnel) handleSSHChannels(chans <-chan ssh.NewChannel) {
	for ch := range chans {
		go t.handleSSHChannel(ch)
	}
}

func (t *Tunnel) handleSSHChannel(ch ssh.NewChannel) {
	hostPort := string(ch.ExtraData())
	//check ACL against the actual requested destination
	if t.Config.ACL != nil && !t.Config.ACL(hostPort) {
		t.Debugf("Denied connection to %s (ACL)", hostPort)
		ch.Reject(ssh.Prohibited, "access denied")
		return
	}
	sshChan, reqs, err := ch.Accept()
	if err != nil {
		t.Debugf("Failed to accept stream: %s", err)
		return
	}
	stream := io.ReadWriteCloser(sshChan)
	defer stream.Close()
	go ssh.DiscardRequests(reqs)
	l := t.Logger.Fork("conn#%d", t.connStats.New())
	t.connStats.Open()
	l.Debugf("Open %s", t.connStats.String())
	var handleErr error
	handleErr = t.handleTCP(l, stream, hostPort)
	t.connStats.Close()
	errmsg := ""
	if handleErr != nil && !strings.HasSuffix(handleErr.Error(), "EOF") {
		errmsg = fmt.Sprintf(" (error %s)", handleErr)
	}
	l.Debugf("Close %s%s", t.connStats.String(), errmsg)
}

func (t *Tunnel) handleSocks(l *cio.Logger, src net.Conn) error {
	return t.socksServer.ServeConn(src)
}

func (t *Tunnel) handleTCP(l *cio.Logger, src io.ReadWriteCloser, hostPort string) error {
	dst, err := net.Dial("tcp", hostPort)
	if err != nil {
		return err
	}
	s, r := cio.Pipe(src, dst)
	l.Debugf("sent %s received %s", sizestr.ToString(s), sizestr.ToString(r))
	return nil
}