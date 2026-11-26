// Package releasepush encrypts browser subscriptions at rest and sends generic
// release update notices using the browser Web Push protocol.
package releasepush

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

var ErrConfiguration = errors.New("invalid browser push configuration")
var ErrSubscription = errors.New("invalid browser push subscription")

type Config struct {
	PublicKey  string
	PrivateKey string
	Contact    string
	StorageKey string
}

func (Config) String() string     { return "browser push configuration (redacted)" }
func (c Config) GoString() string { return c.String() }

type Keys struct {
	Auth   string `json:"auth"`
	P256DH string `json:"p256dh"`
}

type Subscription struct {
	Endpoint string `json:"endpoint"`
	Keys     Keys   `json:"keys"`
}

func (Subscription) String() string     { return "browser push subscription (redacted)" }
func (s Subscription) GoString() string { return s.String() }

type Sender struct {
	publicKey string
	templates webpush.Options
	aead      cipher.AEAD
}

func (*Sender) String() string     { return "browser push sender (redacted)" }
func (s *Sender) GoString() string { return s.String() }

func New(config Config) (*Sender, error) {
	public, err := base64.RawURLEncoding.Strict().DecodeString(config.PublicKey)
	if err != nil || base64.RawURLEncoding.EncodeToString(public) != config.PublicKey {
		return nil, ErrConfiguration
	}
	private, err := base64.RawURLEncoding.Strict().DecodeString(config.PrivateKey)
	if err != nil || base64.RawURLEncoding.EncodeToString(private) != config.PrivateKey {
		return nil, ErrConfiguration
	}
	key, err := ecdh.P256().NewPrivateKey(private)
	if err != nil || !bytes.Equal(key.PublicKey().Bytes(), public) {
		return nil, ErrConfiguration
	}
	contact, err := url.Parse(config.Contact)
	if err != nil || contact.Scheme != "https" || contact.Hostname() == "" || contact.User != nil ||
		contact.RawQuery != "" || contact.ForceQuery || contact.Fragment != "" || len(config.Contact) > 254 {
		return nil, ErrConfiguration
	}
	storage, err := base64.StdEncoding.Strict().DecodeString(config.StorageKey)
	if err != nil || len(storage) != 32 || base64.StdEncoding.EncodeToString(storage) != config.StorageKey || bytes.Equal(storage, private) {
		return nil, ErrConfiguration
	}
	block, err := aes.NewCipher(storage)
	if err != nil {
		return nil, ErrConfiguration
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrConfiguration
	}
	transport := &http.Transport{
		DialContext:           dialProvider,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   4 * time.Second,
		ResponseHeaderTimeout: 4 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		MaxIdleConns:          4,
		MaxIdleConnsPerHost:   1,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &Sender{
		publicKey: config.PublicKey,
		templates: webpush.Options{
			HTTPClient: client, Subscriber: config.Contact, VAPIDPublicKey: config.PublicKey,
			VAPIDPrivateKey: config.PrivateKey, TTL: 3600, Urgency: webpush.UrgencyNormal, Topic: "anime-release",
		},
		aead: aead,
	}, nil
}

func (s *Sender) PublicKey() string { return s.publicKey }

func providerPath(host string) string {
	switch host {
	case "fcm.googleapis.com":
		return "/fcm/send/"
	case "updates.push.services.mozilla.com":
		return "/wpush/v2/"
	case "web.push.apple.com":
		return "/"
	default:
		return ""
	}
}

func (s Subscription) valid() bool {
	if len(s.Endpoint) > 2048 || len(s.Keys.Auth) != 22 || len(s.Keys.P256DH) != 87 {
		return false
	}
	u, err := url.Parse(s.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host != u.Hostname() || u.User != nil ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" {
		return false
	}
	prefix := providerPath(u.Host)
	if prefix == "" || !strings.HasPrefix(u.Path, prefix) || len(u.Path) <= len(prefix) {
		return false
	}
	auth, err := base64.RawURLEncoding.Strict().DecodeString(s.Keys.Auth)
	if err != nil || len(auth) != 16 {
		return false
	}
	key, err := base64.RawURLEncoding.Strict().DecodeString(s.Keys.P256DH)
	if err != nil {
		return false
	}
	_, err = ecdh.P256().NewPublicKey(key)
	return err == nil
}

func dialProvider(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != "443" || providerPath(host) == "" {
		return nil, ErrSubscription
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, ErrSubscription
	}
	dialer := &net.Dialer{Timeout: 4 * time.Second}
	for _, resolved := range addresses {
		ip := resolved.Unmap()
		if !publicProviderIP(ip) {
			continue
		}
		connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return connection, nil
		}
	}
	return nil, ErrSubscription
}

var sharedIPv4 = netip.MustParsePrefix("100.64.0.0/10")

func publicProviderIP(ip netip.Addr) bool {
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !sharedIPv4.Contains(ip)
}

func (s *Sender) Seal(owner string, subscription Subscription) ([]byte, error) {
	if owner == "" || !subscription.valid() {
		return nil, ErrSubscription
	}
	body, err := json.Marshal(subscription)
	if err != nil {
		return nil, ErrSubscription
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, ErrSubscription
	}
	return s.aead.Seal(nonce, nonce, body, []byte("anime-browser-push:v1:"+owner)), nil
}

func (s *Sender) Open(owner string, encrypted []byte) (Subscription, error) {
	var subscription Subscription
	size := s.aead.NonceSize()
	if owner == "" || len(encrypted) < size+s.aead.Overhead() || len(encrypted) > 4096 {
		return subscription, ErrSubscription
	}
	body, err := s.aead.Open(nil, encrypted[:size], encrypted[size:], []byte("anime-browser-push:v1:"+owner))
	if err != nil || json.Unmarshal(body, &subscription) != nil || !subscription.valid() {
		return Subscription{}, ErrSubscription
	}
	return subscription, nil
}

type Delivery int

const (
	Retry Delivery = iota
	Delivered
	Gone
)

func (s *Sender) Send(ctx context.Context, subscription Subscription) (Delivery, time.Duration) {
	if !subscription.valid() {
		return Gone, 0
	}
	// The receiver sees a generic notice. No title, account, event or credential
	// is placed inside the browser message.
	response, err := webpush.SendNotificationWithContext(ctx, []byte("release"), &webpush.Subscription{
		Endpoint: subscription.Endpoint,
		Keys:     webpush.Keys{Auth: subscription.Keys.Auth, P256dh: subscription.Keys.P256DH},
	}, &s.templates)
	if err != nil {
		return Retry, 0
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusCreated, http.StatusAccepted, http.StatusNoContent, http.StatusOK:
		return Delivered, 0
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return Retry, retryDelay(response.Header.Get("Retry-After"), time.Now())
	case http.StatusNotFound, http.StatusGone:
		return Gone, 0
	default:
		return Retry, 0
	}
}

func retryDelay(value string, now time.Time) time.Duration {
	const maxDelay = 24 * time.Hour
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseUint(value, 10, 64); err == nil {
		return time.Duration(min(seconds, uint64(maxDelay/time.Second))) * time.Second
	} else if errors.Is(err, strconv.ErrRange) {
		return maxDelay
	}
	if until, err := http.ParseTime(value); err == nil {
		return min(max(0, until.Sub(now)), maxDelay)
	}
	return 0
}
