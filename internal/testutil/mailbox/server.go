// Package mailbox is a loopback-only SMTP/HTTP fixture for synthetic tests. It
// is not imported by the API and must never be used for real account delivery.
package mailbox

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"strings"
	"sync"
	"time"
)

type Message struct {
	Recipient string `json:"recipient"`
	Body      string `json:"body"`
}
type Server struct {
	smtp, web           net.Listener
	http                *http.Server
	mu                  sync.Mutex
	messages            []Message
	conns               map[net.Conn]bool
	closed              bool
	wg                  sync.WaitGroup
	user, password, key string
	CAPEM               []byte
}

func listen(addr string) (net.Listener, error) {
	host, _, err := net.SplitHostPort(addr)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsLoopback() {
		return nil, errors.New("mail fixture requires a numeric loopback address")
	}
	return net.Listen("tcp", addr)
}
func New(smtpAddr, httpAddr, user, password, key string) (*Server, error) {
	return NewWithCAFile(smtpAddr, httpAddr, user, password, key, "")
}
func NewWithCAFile(smtpAddr, httpAddr, user, password, key, caFile string) (*Server, error) {
	if user == "" || password == "" || len(key) < 32 {
		return nil, errors.New("mail fixture requires explicit credentials")
	}
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	cert := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Synthetic mail fixture"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(24 * time.Hour), KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true, IsCA: true, IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}, DNSNames: []string{"localhost"}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &private.PublicKey, private)
	if err != nil {
		return nil, err
	}
	publicCA := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if caFile != "" {
		if err = os.WriteFile(caFile, publicCA, 0600); err != nil {
			return nil, err
		}
	}
	pair := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: private}
	smtpListener, err := listen(smtpAddr)
	if err != nil {
		return nil, err
	}
	web, err := listen(httpAddr)
	if err != nil {
		smtpListener.Close()
		return nil, err
	}
	s := &Server{smtp: tls.NewListener(smtpListener, &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pair}}), web: web, user: user, password: password, key: key, conns: map[net.Conn]bool{}, CAPEM: publicCA}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("GET /messages", func(w http.ResponseWriter, r *http.Request) {
		actual, expected := sha256.Sum256([]byte(r.Header.Get("X-Test-Mailbox-Key"))), sha256.Sum256([]byte(s.key))
		if subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		s.mu.Lock()
		out := []Message{}
		for _, m := range s.messages {
			if m.Recipient == r.URL.Query().Get("recipient") {
				out = append(out, m)
			}
		}
		s.mu.Unlock()
		_ = json.NewEncoder(w).Encode(out)
	})
	s.http = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
	s.wg.Add(2)
	go func() { defer s.wg.Done(); _ = s.http.Serve(web) }()
	go s.accept()
	return s, nil
}
func (s *Server) SMTPAddress() string { return s.smtp.Addr().String() }
func (s *Server) HTTPAddress() string { return s.web.Addr().String() }
func (s *Server) Messages() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Message(nil), s.messages...)
}
func (s *Server) accept() {
	defer s.wg.Done()
	for {
		conn, err := s.smtp.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			conn.Close()
			return
		}
		s.conns[conn] = true
		s.wg.Add(1)
		s.mu.Unlock()
		go s.serve(conn)
	}
}
func (s *Server) serve(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()
	defer func() { s.mu.Lock(); delete(s.conns, conn); s.mu.Unlock() }()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	reader := textproto.NewReader(bufio.NewReader(conn))
	writer := bufio.NewWriter(conn)
	reply := func(text string) bool {
		_, err := writer.WriteString(text + "\r\n")
		if err != nil {
			return false
		}
		return writer.Flush() == nil
	}
	if !reply("220 localhost ESMTP fixture") {
		return
	}
	authenticated := false
	recipient := ""
	for {
		line, err := reader.ReadLine()
		if err != nil {
			return
		}
		switch {
		case strings.HasPrefix(line, "EHLO"):
			if !reply("250-localhost\r\n250-AUTH PLAIN\r\n250 8BITMIME") {
				return
			}
		case strings.HasPrefix(line, "AUTH PLAIN "):
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(line, "AUTH PLAIN "))
			parts := strings.Split(string(decoded), "\x00")
			authenticated = err == nil && len(parts) == 3 && parts[1] == s.user && parts[2] == s.password
			if authenticated {
				if !reply("235 authenticated") {
					return
				}
			} else if !reply("535 rejected") {
				return
			}
		case strings.HasPrefix(line, "MAIL FROM:"):
			if !authenticated {
				if !reply("530 authentication required") {
					return
				}
			} else if !reply("250 ok") {
				return
			}
		case strings.HasPrefix(line, "RCPT TO:"):
			if !authenticated {
				if !reply("530 authentication required") {
					return
				}
				continue
			}
			a, b := strings.Index(line, "<"), strings.Index(line, ">")
			if a < 0 || b <= a {
				if !reply("501 invalid recipient") {
					return
				}
				continue
			}
			recipient = line[a+1 : b]
			if !reply("250 ok") {
				return
			}
		case line == "DATA":
			if !authenticated || recipient == "" {
				if !reply("530 authentication required") {
					return
				}
				continue
			}
			if !reply("354 send message") {
				return
			}
			data, err := io.ReadAll(io.LimitReader(reader.DotReader(), 65537))
			if err != nil || len(data) > 65536 {
				return
			}
			s.mu.Lock()
			if len(s.messages) < 100 {
				s.messages = append(s.messages, Message{recipient, string(data)})
			}
			s.mu.Unlock()
			recipient = ""
			if !reply("250 accepted") {
				return
			}
		case line == "QUIT":
			reply("221 bye")
			return
		case line == "RSET":
			recipient = ""
			if !reply("250 ok") {
				return
			}
		default:
			if !reply("250 ok") {
				return
			}
		}
	}
}
func (s *Server) Close() {
	s.mu.Lock()
	s.closed = true
	for conn := range s.conns {
		_ = conn.Close()
	}
	s.mu.Unlock()
	_ = s.smtp.Close()
	_ = s.http.Close()
	s.wg.Wait()
}
