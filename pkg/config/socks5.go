package config

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// socks5Dialer returns a DialContext function that tunnels TCP connections
// through the SOCKS5 proxy described by u (socks5://[user:pass@]host:port).
// The standard library http.Transport only supports http/https proxies, so
// socks5 URLs need a custom dialer.
func socks5Dialer(u *url.URL) func(ctx context.Context, network, addr string) (net.Conn, error) {
	proxyAddr := u.Host
	var user, pass string
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		d := net.Dialer{}
		conn, err := d.DialContext(ctx, "tcp", proxyAddr)
		if err != nil {
			return nil, err
		}
		if err := socks5Handshake(conn, user, pass, addr); err != nil {
			conn.Close()
			return nil, err
		}
		return conn, nil
	}
}

// socks5Handshake performs the SOCKS5 greeting, optional username/password
// authentication (RFC 1929) and a CONNECT request to target (RFC 1928).
func socks5Handshake(conn net.Conn, user, pass, target string) error {
	methods := []byte{0x00} // no authentication
	if user != "" {
		methods = append(methods, 0x02) // also offer user/password
	}
	greet := append([]byte{0x05, byte(len(methods))}, methods...)
	if _, err := conn.Write(greet); err != nil {
		return err
	}
	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return err
	}
	if resp[0] != 0x05 {
		return errors.New("socks5: unsupported protocol version")
	}
	switch resp[1] {
	case 0x00: // no auth accepted
	case 0x02:
		if err := socks5Auth(conn, user, pass); err != nil {
			return err
		}
	default:
		return fmt.Errorf("socks5: no acceptable auth method (server chose %d)", resp[1])
	}

	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 0 || port > 65535 {
		return fmt.Errorf("socks5: invalid target port %q", portStr)
	}

	req := []byte{0x05, 0x01, 0x00}
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			req = append(req, 0x01)
			req = append(req, ip4...)
		} else {
			req = append(req, 0x04)
			req = append(req, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return errors.New("socks5: target host name too long")
		}
		req = append(req, 0x03, byte(len(host)))
		req = append(req, host...)
	}
	var p [2]byte
	binary.BigEndian.PutUint16(p[:], uint16(port))
	req = append(req, p[:]...)
	if _, err := conn.Write(req); err != nil {
		return err
	}

	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		return err
	}
	if head[0] != 0x05 {
		return errors.New("socks5: invalid reply version")
	}
	if head[1] != 0x00 {
		return fmt.Errorf("socks5: connect failed (code %d)", head[1])
	}
	var addrLen int
	switch head[3] {
	case 0x01:
		addrLen = net.IPv4len
	case 0x04:
		addrLen = net.IPv6len
	case 0x03:
		b := make([]byte, 1)
		if _, err := io.ReadFull(conn, b); err != nil {
			return err
		}
		addrLen = int(b[0])
	default:
		return errors.New("socks5: invalid reply address type")
	}
	if _, err := io.CopyN(io.Discard, conn, int64(addrLen+2)); err != nil {
		return err
	}
	return nil
}

// socks5Auth performs RFC 1929 username/password authentication.
func socks5Auth(conn net.Conn, user, pass string) error {
	if user == "" {
		return errors.New("socks5: proxy requires credentials")
	}
	if len(user) > 255 || len(pass) > 255 {
		return errors.New("socks5: credentials too long")
	}
	req := []byte{0x01, byte(len(user))}
	req = append(req, user...)
	req = append(req, byte(len(pass)))
	req = append(req, pass...)
	if _, err := conn.Write(req); err != nil {
		return err
	}
	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return err
	}
	if resp[0] != 0x01 || resp[1] != 0x00 {
		return errors.New("socks5: authentication failed")
	}
	return nil
}

// isSocks5Scheme reports whether a proxy URL uses the socks5 scheme.
func isSocks5Scheme(u *url.URL) bool {
	return u != nil && strings.EqualFold(u.Scheme, "socks5")
}
