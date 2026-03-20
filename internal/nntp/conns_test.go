package nntp

import (
	"bufio"
	"fmt"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"
)

// fakeServer is a minimal NNTP server backed by net.Listener.
// Each handler receives the raw connection and is responsible for the
// full protocol conversation (greeting, commands, quit).
type fakeServer struct {
	ln      net.Listener
	handler func(conn net.Conn)
	wg      sync.WaitGroup
}

func newFakeServer(t *testing.T, handler func(conn net.Conn)) *fakeServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &fakeServer{ln: ln, handler: handler}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			c, err := ln.Accept()
			if err != nil {
				return // listener closed
			}
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				defer c.Close()
				handler(c)
			}()
		}
	}()
	return s
}

func (s *fakeServer) addr() string { return s.ln.Addr().String() }

func (s *fakeServer) close() {
	s.ln.Close()
	s.wg.Wait()
}

// newTestConnection dials addr and wraps the net.Conn in a Connection
// with buffered I/O identical to production, minus TLS/logging.
func newTestConnection(t *testing.T, addr, user, pass string) *Connection {
	t.Helper()
	raw, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	reader := bufio.NewReaderSize(raw, 64*1024)
	return &Connection{
		username: user,
		password: pass,
		conn:     raw,
		reader:   reader,
		writer:   bufio.NewWriterSize(raw, 64*1024),
		text:     textproto.NewReader(reader),
	}
}

// reply writes an NNTP response line (CRLF‑terminated) to w.
func reply(w *bufio.Writer, line string) {
	fmt.Fprintf(w, "%s\r\n", line)
	w.Flush()
}

// readLine reads one CRLF-terminated line from the scanner.
func readLine(sc *bufio.Scanner) string {
	if sc.Scan() {
		return sc.Text()
	}
	return ""
}

// ---------- Tests ----------

func TestNNTP_ConnectAndQuit(t *testing.T) {
	var gotQuit bool

	srv := newFakeServer(t, func(c net.Conn) {
		w := bufio.NewWriter(c)
		sc := bufio.NewScanner(c)

		// Greeting
		reply(w, "200 fake-server ready")

		// Wait for QUIT
		for sc.Scan() {
			cmd := sc.Text()
			if strings.HasPrefix(cmd, "QUIT") {
				gotQuit = true
				reply(w, "205 bye")
				return
			}
		}
	})
	defer srv.close()

	conn := newTestConnection(t, srv.addr(), "", "")
	defer conn.Close()

	// Read greeting
	resp, err := conn.readResponse()
	if err != nil {
		t.Fatalf("greeting: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("expected 200 greeting, got %d", resp.Code)
	}

	// Send QUIT
	if err := conn.sendCommand("QUIT"); err != nil {
		t.Fatalf("send QUIT: %v", err)
	}
	resp, err = conn.readResponse()
	if err != nil {
		t.Fatalf("read QUIT response: %v", err)
	}
	if resp.Code != 205 {
		t.Fatalf("expected 205, got %d", resp.Code)
	}

	conn.Close()
	srv.close()

	if !gotQuit {
		t.Fatal("server never saw QUIT command")
	}
}

func TestNNTP_AuthInfo(t *testing.T) {
	var commands []string

	srv := newFakeServer(t, func(c net.Conn) {
		w := bufio.NewWriter(c)
		sc := bufio.NewScanner(c)

		reply(w, "200 ready - auth required")

		for sc.Scan() {
			cmd := sc.Text()
			commands = append(commands, cmd)

			switch {
			case strings.HasPrefix(cmd, "AUTHINFO USER"):
				reply(w, "381 PASS required")
			case strings.HasPrefix(cmd, "AUTHINFO PASS"):
				reply(w, "281 Authentication accepted")
			case strings.HasPrefix(cmd, "QUIT"):
				reply(w, "205 bye")
				return
			}
		}
	})
	defer srv.close()

	conn := newTestConnection(t, srv.addr(), "testuser", "testpass")
	defer conn.Close()

	// Read greeting
	_, err := conn.readResponse()
	if err != nil {
		t.Fatalf("greeting: %v", err)
	}

	// Authenticate
	if err := conn.authenticate(); err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	// Verify command sequence
	if len(commands) < 2 {
		t.Fatalf("expected at least 2 commands, got %d", len(commands))
	}
	if commands[0] != "AUTHINFO USER testuser" {
		t.Errorf("command[0] = %q, want AUTHINFO USER testuser", commands[0])
	}
	if commands[1] != "AUTHINFO PASS testpass" {
		t.Errorf("command[1] = %q, want AUTHINFO PASS testpass", commands[1])
	}
}

func TestNNTP_AuthInfo_Rejected(t *testing.T) {
	srv := newFakeServer(t, func(c net.Conn) {
		w := bufio.NewWriter(c)
		sc := bufio.NewScanner(c)

		reply(w, "200 ready")

		for sc.Scan() {
			cmd := sc.Text()
			switch {
			case strings.HasPrefix(cmd, "AUTHINFO USER"):
				reply(w, "381 PASS required")
			case strings.HasPrefix(cmd, "AUTHINFO PASS"):
				reply(w, "481 Authentication failed")
			}
		}
	})
	defer srv.close()

	conn := newTestConnection(t, srv.addr(), "bad", "creds")
	defer conn.Close()

	_, _ = conn.readResponse()

	err := conn.authenticate()
	if err == nil {
		t.Fatal("expected authentication error, got nil")
	}
	var nntpErr *Error
	if !isError(err, &nntpErr) {
		t.Fatalf("expected *nntp.Error, got %T", err)
	}
	if nntpErr.Type != ErrorTypeAuthentication {
		t.Errorf("error type = %v, want ErrorTypeAuthentication", nntpErr.Type)
	}
}

func TestNNTP_FetchArticle(t *testing.T) {
	articleLines := []string{
		"Subject: Test Article",
		"From: poster@example.com",
		"Date: Thu, 12 Mar 2026 06:00:00 +0000",
		"Newsgroups: alt.test",
		"Message-ID: <abc123@example.com>",
		"",
		"This is the body.",
		"Second line of body.",
	}

	srv := newFakeServer(t, func(c net.Conn) {
		w := bufio.NewWriter(c)
		sc := bufio.NewScanner(c)

		reply(w, "200 ready")

		for sc.Scan() {
			cmd := sc.Text()
			switch {
			case strings.HasPrefix(cmd, "ARTICLE"):
				// Multi-line: status line, then dot-terminated body
				reply(w, "220 0 <abc123@example.com> article retrieved")
				for _, l := range articleLines {
					fmt.Fprintf(w, "%s\r\n", l)
				}
				fmt.Fprintf(w, ".\r\n")
				w.Flush()
			case strings.HasPrefix(cmd, "QUIT"):
				reply(w, "205 bye")
				return
			}
		}
	})
	defer srv.close()

	conn := newTestConnection(t, srv.addr(), "", "")
	defer conn.Close()

	_, _ = conn.readResponse()

	article, err := conn.GetArticle("abc123@example.com")
	if err != nil {
		t.Fatalf("GetArticle: %v", err)
	}

	if article.Subject != "Test Article" {
		t.Errorf("Subject = %q, want %q", article.Subject, "Test Article")
	}
	if article.From != "poster@example.com" {
		t.Errorf("From = %q, want %q", article.From, "poster@example.com")
	}
	if article.Date != "Thu, 12 Mar 2026 06:00:00 +0000" {
		t.Errorf("Date = %q", article.Date)
	}
	if len(article.Groups) != 1 || article.Groups[0] != "alt.test" {
		t.Errorf("Groups = %v, want [alt.test]", article.Groups)
	}

	wantBody := "This is the body.\nSecond line of body."
	if string(article.Body) != wantBody {
		t.Errorf("Body = %q, want %q", string(article.Body), wantBody)
	}
}

func TestNNTP_ErrorHandling_ArticleNotFound(t *testing.T) {
	srv := newFakeServer(t, func(c net.Conn) {
		w := bufio.NewWriter(c)
		sc := bufio.NewScanner(c)

		reply(w, "200 ready")

		for sc.Scan() {
			cmd := sc.Text()
			switch {
			case strings.HasPrefix(cmd, "ARTICLE"):
				reply(w, "430 No Such Article Found")
			case strings.HasPrefix(cmd, "QUIT"):
				reply(w, "205 bye")
				return
			}
		}
	})
	defer srv.close()

	conn := newTestConnection(t, srv.addr(), "", "")
	defer conn.Close()

	_, _ = conn.readResponse()

	_, err := conn.GetArticle("missing@example.com")
	if err == nil {
		t.Fatal("expected error for missing article")
	}

	if !IsArticleNotFoundError(err) {
		t.Errorf("IsArticleNotFoundError = false, want true; err = %v", err)
	}

	var nntpErr *Error
	if isError(err, &nntpErr) {
		if nntpErr.Code != 430 {
			t.Errorf("error code = %d, want 430", nntpErr.Code)
		}
	}
}

func TestNNTP_ErrorHandling_ServerBusy(t *testing.T) {
	srv := newFakeServer(t, func(c net.Conn) {
		w := bufio.NewWriter(c)
		sc := bufio.NewScanner(c)

		reply(w, "200 ready")

		for sc.Scan() {
			cmd := sc.Text()
			switch {
			case strings.HasPrefix(cmd, "ARTICLE"):
				reply(w, "400 Server temporarily unavailable")
			case strings.HasPrefix(cmd, "QUIT"):
				reply(w, "205 bye")
				return
			}
		}
	})
	defer srv.close()

	conn := newTestConnection(t, srv.addr(), "", "")
	defer conn.Close()

	_, _ = conn.readResponse()

	_, err := conn.GetArticle("any@example.com")
	if err == nil {
		t.Fatal("expected error for busy server")
	}

	var nntpErr *Error
	if !isError(err, &nntpErr) {
		t.Fatalf("expected *nntp.Error, got %T", err)
	}
	if nntpErr.Type != ErrorTypeServerBusy {
		t.Errorf("error type = %v, want ErrorTypeServerBusy", nntpErr.Type)
	}
	if !nntpErr.IsRetryable() {
		t.Error("400 error should be retryable")
	}
}

func TestNNTP_Stat(t *testing.T) {
	srv := newFakeServer(t, func(c net.Conn) {
		w := bufio.NewWriter(c)
		sc := bufio.NewScanner(c)

		reply(w, "200 ready")

		for sc.Scan() {
			cmd := sc.Text()
			switch {
			case strings.HasPrefix(cmd, "STAT"):
				reply(w, "223 42 <test@example.com>")
			case strings.HasPrefix(cmd, "QUIT"):
				reply(w, "205 bye")
				return
			}
		}
	})
	defer srv.close()

	conn := newTestConnection(t, srv.addr(), "", "")
	defer conn.Close()

	_, _ = conn.readResponse()

	num, echoedID, err := conn.Stat("test@example.com")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if num != 42 {
		t.Errorf("article number = %d, want 42", num)
	}
	if echoedID != "<test@example.com>" {
		t.Errorf("echoed ID = %q, want <test@example.com>", echoedID)
	}
}

func TestNNTP_PipelinedStat(t *testing.T) {
	srv := newFakeServer(t, func(c net.Conn) {
		w := bufio.NewWriter(c)
		sc := bufio.NewScanner(c)

		reply(w, "200 ready")

		for sc.Scan() {
			cmd := sc.Text()
			switch {
			case strings.HasPrefix(cmd, "STAT <found@example.com>"):
				reply(w, "223 1 <found@example.com>")
			case strings.HasPrefix(cmd, "STAT <missing@example.com>"):
				reply(w, "430 No Such Article")
			case strings.HasPrefix(cmd, "STAT <also-found@example.com>"):
				reply(w, "223 3 <also-found@example.com>")
			case strings.HasPrefix(cmd, "QUIT"):
				reply(w, "205 bye")
				return
			}
		}
	})
	defer srv.close()

	conn := newTestConnection(t, srv.addr(), "", "")
	defer conn.Close()

	_, _ = conn.readResponse()

	ids := []string{"found@example.com", "missing@example.com", "also-found@example.com"}
	results, err := conn.PipelinedStat(ids)
	if err != nil {
		t.Fatalf("PipelinedStat: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	if !results[0].Available {
		t.Error("results[0] should be available")
	}
	if results[1].Available {
		t.Error("results[1] should NOT be available")
	}
	if !results[2].Available {
		t.Error("results[2] should be available")
	}
}

func TestNNTP_Ping(t *testing.T) {
	srv := newFakeServer(t, func(c net.Conn) {
		w := bufio.NewWriter(c)
		sc := bufio.NewScanner(c)

		reply(w, "200 ready")

		for sc.Scan() {
			cmd := sc.Text()
			switch {
			case strings.HasPrefix(cmd, "DATE"):
				reply(w, "111 20260312060000")
			case strings.HasPrefix(cmd, "QUIT"):
				reply(w, "205 bye")
				return
			}
		}
	})
	defer srv.close()

	conn := newTestConnection(t, srv.addr(), "", "")
	defer conn.Close()

	_, _ = conn.readResponse()

	if err := conn.ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestNNTP_FormatMessageID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abc@example.com", "<abc@example.com>"},
		{"<abc@example.com>", "<abc@example.com>"},
		{"<abc@example.com", "<abc@example.com>"},
		{"abc@example.com>", "<abc@example.com>"},
		{" abc@example.com ", "<abc@example.com>"},
	}

	for _, tt := range tests {
		got := FormatMessageID(tt.input)
		if got != tt.want {
			t.Errorf("FormatMessageID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNNTP_CloseIdempotent(t *testing.T) {
	srv := newFakeServer(t, func(c net.Conn) {
		w := bufio.NewWriter(c)
		reply(w, "200 ready")
		// Just hold the connection open
		sc := bufio.NewScanner(c)
		for sc.Scan() {
		}
	})
	defer srv.close()

	conn := newTestConnection(t, srv.addr(), "", "")

	_, _ = conn.readResponse()

	if conn.IsClosed() {
		t.Fatal("connection should not be closed yet")
	}

	// Close twice — must not panic or return error on second call
	if err := conn.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if !conn.IsClosed() {
		t.Fatal("connection should be closed after Close()")
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("second Close should be nil, got: %v", err)
	}
}

// isError is a thin wrapper so we don't import errors in tests for As.
func isError(err error, target interface{}) bool {
	switch t := target.(type) {
	case **Error:
		for err != nil {
			if e, ok := err.(*Error); ok {
				*t = e
				return true
			}
			if u, ok := err.(interface{ Unwrap() error }); ok {
				err = u.Unwrap()
			} else {
				return false
			}
		}
	}
	return false
}
