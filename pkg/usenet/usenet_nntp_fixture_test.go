package usenet

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/sirrobot01/decypharr/internal/nntp"
)

type testNNTPServer struct {
	t          *testing.T
	ln         net.Listener
	user       string
	pass       string
	articles   map[string][]byte
	bodyEvents chan string
	blockBody  map[string]chan struct{}
	dropBody   map[string]int
	mu         sync.RWMutex
}

func startTestNNTPServer(t *testing.T, user, pass string, articles map[string][]byte) *testNNTPServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	s := &testNNTPServer{
		t:          t,
		ln:         ln,
		user:       user,
		pass:       pass,
		articles:   make(map[string][]byte, len(articles)),
		bodyEvents: make(chan string, 128),
		blockBody:  make(map[string]chan struct{}),
		dropBody:   make(map[string]int),
	}
	for k, v := range articles {
		s.articles[nntp.FormatMessageID(k)] = v
	}

	go s.serve()
	t.Cleanup(func() { _ = s.ln.Close() })
	return s
}

func (s *testNNTPServer) hostPort() (string, int) {
	host, portStr, err := net.SplitHostPort(s.ln.Addr().String())
	if err != nil {
		s.t.Fatalf("split host/port: %v", err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		s.t.Fatalf("parse port: %v", err)
	}
	return host, port
}

func (s *testNNTPServer) setBodyBlock(messageID string, ch chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blockBody[nntp.FormatMessageID(messageID)] = ch
}

func (s *testNNTPServer) setBodyDrop(messageID string, bytes int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropBody[nntp.FormatMessageID(messageID)] = bytes
}

func (s *testNNTPServer) waitBodyRequest(t *testing.T, want string) {
	t.Helper()
	for {
		got := <-s.bodyEvents
		if got == nntp.FormatMessageID(want) {
			return
		}
	}
}

func (s *testNNTPServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *testNNTPServer) handleConn(conn net.Conn) {
	defer conn.Close()
	br := bufio.NewReader(conn)
	bw := bufio.NewWriter(conn)

	_, _ = bw.WriteString("200 test nntp ready\r\n")
	_ = bw.Flush()

	authOK := false
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				s.t.Logf("nntp read error: %v", err)
			}
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		cmd := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(cmd, "AUTHINFO USER "):
			user := strings.TrimSpace(line[len("AUTHINFO USER "):])
			if user != s.user {
				_, _ = bw.WriteString("481 bad user\r\n")
			} else {
				_, _ = bw.WriteString("381 pass required\r\n")
			}
			_ = bw.Flush()

		case strings.HasPrefix(cmd, "AUTHINFO PASS "):
			pass := strings.TrimSpace(line[len("AUTHINFO PASS "):])
			if pass != s.pass {
				_, _ = bw.WriteString("481 bad pass\r\n")
			} else {
				authOK = true
				_, _ = bw.WriteString("281 auth accepted\r\n")
			}
			_ = bw.Flush()

		case strings.HasPrefix(cmd, "STAT "):
			if !authOK {
				_, _ = bw.WriteString("480 auth required\r\n")
				_ = bw.Flush()
				continue
			}
			msgID := nntp.FormatMessageID(strings.TrimSpace(line[len("STAT "):]))
			if s.articleExists(msgID) {
				_, _ = bw.WriteString(fmt.Sprintf("223 1 %s\r\n", msgID))
			} else {
				_, _ = bw.WriteString("430 no such article\r\n")
			}
			_ = bw.Flush()

		case strings.HasPrefix(cmd, "BODY "):
			if !authOK {
				_, _ = bw.WriteString("480 auth required\r\n")
				_ = bw.Flush()
				continue
			}
			msgID := nntp.FormatMessageID(strings.TrimSpace(line[len("BODY "):]))
			body, ok := s.articleBody(msgID)
			if !ok {
				_, _ = bw.WriteString("430 no such article\r\n")
				_ = bw.Flush()
				continue
			}
			s.bodyEvents <- msgID

			if ch := s.bodyBlocker(msgID); ch != nil {
				<-ch
			}

			_, _ = bw.WriteString(fmt.Sprintf("222 0 %s\r\n", msgID))

			if dropBytes, drop := s.bodyDropBytes(msgID); drop {
				if dropBytes < 0 {
					dropBytes = 0
				}
				if dropBytes > len(body) {
					dropBytes = len(body)
				}
				_, _ = bw.Write(body[:dropBytes])
				_ = bw.Flush()
				return
			}

			_, _ = bw.Write(body)
			_, _ = bw.WriteString(".\r\n")
			_ = bw.Flush()

		case cmd == "DATE":
			_, _ = bw.WriteString("111 20260101000000\r\n")
			_ = bw.Flush()

		case cmd == "QUIT":
			_, _ = bw.WriteString("205 bye\r\n")
			_ = bw.Flush()
			return

		default:
			_, _ = bw.WriteString("500 unknown command\r\n")
			_ = bw.Flush()
		}
	}
}

func (s *testNNTPServer) articleExists(messageID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.articles[messageID]
	return ok
}

func (s *testNNTPServer) articleBody(messageID string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.articles[messageID]
	if !ok {
		return nil, false
	}
	return b, true
}

func (s *testNNTPServer) bodyBlocker(messageID string) chan struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.blockBody[messageID]
}

func (s *testNNTPServer) bodyDropBytes(messageID string) (int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.dropBody[messageID]
	return v, ok
}

func yencEncodeForNNTP(data []byte, name string, partNum int, begin, end int64) []byte {
	var buf bytes.Buffer

	if partNum > 0 {
		buf.WriteString(fmt.Sprintf("=ybegin part=%d line=128 size=%d name=%s\r\n", partNum, len(data), name))
		buf.WriteString(fmt.Sprintf("=ypart begin=%d end=%d\r\n", begin, end))
	} else {
		buf.WriteString(fmt.Sprintf("=ybegin line=128 size=%d name=%s\r\n", len(data), name))
	}

	col := 0
	for _, b := range data {
		encoded := (b + 42) & 0xFF
		if encoded == 0 || encoded == '\n' || encoded == '\r' || encoded == '=' || encoded == '\t' || encoded == ' ' || encoded == '.' {
			buf.WriteByte('=')
			buf.WriteByte((encoded + 64) & 0xFF)
			col += 2
		} else {
			buf.WriteByte(encoded)
			col++
		}
		if col >= 128 {
			buf.WriteString("\r\n")
			col = 0
		}
	}
	if col > 0 {
		buf.WriteString("\r\n")
	}

	buf.WriteString(fmt.Sprintf("=yend size=%d\r\n", len(data)))
	return buf.Bytes()
}

func buildSimpleNZB(filename, messageID string, bytes int) []byte {
	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<nzb xmlns="http://www.newzbin.com/DTD/2003/nzb">
  <file poster="tester" date="1700000000" subject="%s yEnc (1/1)">
    <groups>
      <group>alt.binaries.test</group>
    </groups>
    <segments>
      <segment bytes="%d" number="1">%s</segment>
    </segments>
  </file>
</nzb>`, filename, bytes, messageID)
	return []byte(xml)
}
