package nntp

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strings"
	"testing"
)

// yencEncodeTestBody returns a ready-to-send NNTP yEnc body for the given data.
// Includes =ybegin, =ypart, encoded payload, =yend, and NNTP dot terminator.
func yencEncodeTestBody(data []byte, name string) []byte {
	var buf bytes.Buffer
	size := len(data)
	buf.WriteString(fmt.Sprintf("=ybegin part=1 line=128 size=%d name=%s\r\n", size, name))
	buf.WriteString(fmt.Sprintf("=ypart begin=1 end=%d\r\n", size))

	col := 0
	for _, b := range data {
		enc := (b + 42) & 0xFF
		switch enc {
		case 0, '\n', '\r', '=', '\t', ' ', '.':
			buf.WriteByte('=')
			buf.WriteByte((enc + 64) & 0xFF)
			col += 2
		default:
			buf.WriteByte(enc)
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

	buf.WriteString(fmt.Sprintf("=yend size=%d\r\n", size))
	buf.WriteString(".\r\n")
	return buf.Bytes()
}

// TestGetDecodedBody_BinaryRoundTrip verifies that GetDecodedBody decodes all
// 256 byte values without corruption — exercises every encoding escape path.
func TestGetDecodedBody_BinaryRoundTrip(t *testing.T) {
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}
	msgID := "<decode-roundtrip@test>"
	body := yencEncodeTestBody(data, "all-bytes.bin")

	srv := newFakeServer(t, func(c net.Conn) {
		w := bufio.NewWriter(c)
		sc := bufio.NewScanner(c)

		reply(w, "200 ready")

		for sc.Scan() {
			if strings.HasPrefix(strings.ToUpper(sc.Text()), "BODY ") {
				reply(w, fmt.Sprintf("222 0 %s", msgID))
				_, _ = w.Write(body)
				_ = w.Flush()
			}
		}
	})
	defer srv.close()

	conn := newTestConnection(t, srv.addr(), "", "")
	defer conn.Close()

	_, _ = conn.readResponse() // consume greeting

	got, err := conn.GetDecodedBody(msgID)
	if err != nil {
		t.Fatalf("GetDecodedBody: %v", err)
	}
	if len(got) != len(data) {
		t.Fatalf("GetDecodedBody: got %d bytes, want %d", len(got), len(data))
	}
	if !bytes.Equal(got, data) {
		t.Fatal("GetDecodedBody: decoded content does not match original")
	}
}

// TestGetDecodedBody_ArticleNotFound verifies that a 430 response from the server
// surfaces as an article-not-found error rather than being swallowed.
func TestGetDecodedBody_ArticleNotFound(t *testing.T) {
	srv := newFakeServer(t, func(c net.Conn) {
		w := bufio.NewWriter(c)
		sc := bufio.NewScanner(c)

		reply(w, "200 ready")

		for sc.Scan() {
			if strings.HasPrefix(strings.ToUpper(sc.Text()), "BODY ") {
				reply(w, "430 No Such Article")
			}
		}
	})
	defer srv.close()

	conn := newTestConnection(t, srv.addr(), "", "")
	defer conn.Close()

	_, _ = conn.readResponse()

	_, err := conn.GetDecodedBody("<missing@test>")
	if err == nil {
		t.Fatal("GetDecodedBody: expected article-not-found error, got nil")
	}
	if !IsArticleNotFoundError(err) {
		t.Fatalf("GetDecodedBody: expected IsArticleNotFoundError=true, got: %v", err)
	}
}

// TestStreamBody_DecodedBytesMatchInput verifies that StreamBody writes the exact
// decoded bytes to the writer — the same decode path used by SegmentFetcher.
func TestStreamBody_DecodedBytesMatchInput(t *testing.T) {
	data := []byte("Hello from streaming yEnc decoder — multi-byte content")
	msgID := "<stream-body@test>"
	body := yencEncodeTestBody(data, "hello.txt")

	srv := newFakeServer(t, func(c net.Conn) {
		w := bufio.NewWriter(c)
		sc := bufio.NewScanner(c)

		reply(w, "200 ready")

		for sc.Scan() {
			if strings.HasPrefix(strings.ToUpper(sc.Text()), "BODY ") {
				reply(w, fmt.Sprintf("222 0 %s", msgID))
				_, _ = w.Write(body)
				_ = w.Flush()
			}
		}
	})
	defer srv.close()

	conn := newTestConnection(t, srv.addr(), "", "")
	defer conn.Close()

	_, _ = conn.readResponse()

	var out bytes.Buffer
	n, err := conn.StreamBody(msgID, &out)
	if err != nil {
		t.Fatalf("StreamBody: %v", err)
	}
	if n != int64(len(data)) {
		t.Fatalf("StreamBody: returned n=%d, want %d", n, len(data))
	}
	if !bytes.Equal(out.Bytes(), data) {
		t.Fatalf("StreamBody: content mismatch: got %q, want %q", out.Bytes(), data)
	}
}
