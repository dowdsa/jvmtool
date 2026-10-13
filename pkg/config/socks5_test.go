package config

import (
	"bufio"
	"io"
	"net"
	"testing"
	"time"
)

// fakeSocks5 runs a minimal SOCKS5 server on ln, negotiating according to
// the mode: "auth" requires user/pass, "downgrade" always selects no-auth
// regardless of offered methods, "noauth" selects no-auth. It stops after
// completing (or failing) the CONNECT phase.
func fakeSocks5(t *testing.T, ln net.Listener, mode, wantUser, wantPass string) {
	t.Helper()
	conn, err := ln.Accept()
	if err != nil {
		t.Errorf("accept: %v", err)
		return
	}
	defer conn.Close()
	r := bufio.NewReader(conn)

	head := make([]byte, 2)
	if _, err := io.ReadFull(r, head); err != nil {
		return
	}
	if head[0] != 0x05 {
		return
	}
	offered := make([]byte, head[1])
	if _, err := io.ReadFull(r, offered); err != nil {
		return
	}

	chosen := byte(0x02)
	switch mode {
	case "downgrade":
		chosen = 0x00
	case "noauth":
		chosen = 0x00
	}
	if _, err := conn.Write([]byte{0x05, chosen}); err != nil {
		return
	}

	if chosen == 0x02 {
		ver := make([]byte, 1)
		if _, err := io.ReadFull(r, ver); err != nil || ver[0] != 0x01 {
			return
		}
		ulen := make([]byte, 1)
		if _, err := io.ReadFull(r, ulen); err != nil {
			return
		}
		user := make([]byte, ulen[0])
		if _, err := io.ReadFull(r, user); err != nil {
			return
		}
		plen := make([]byte, 1)
		if _, err := io.ReadFull(r, plen); err != nil {
			return
		}
		pass := make([]byte, plen[0])
		if _, err := io.ReadFull(r, pass); err != nil {
			return
		}
		ok := string(user) == wantUser && string(pass) == wantPass
		status := byte(0xff)
		if ok {
			status = 0x00
		}
		if _, err := conn.Write([]byte{0x01, status}); err != nil {
			return
		}
		if !ok {
			return
		}
	}

	// CONNECT request: ver, cmd, rsv, atyp ...
	req := make([]byte, 4)
	if _, err := io.ReadFull(r, req); err != nil {
		return
	}
	switch req[3] {
	case 0x01: // IPv4
		io.CopyN(io.Discard, r, 4+2)
	case 0x03: // domain
		l := make([]byte, 1)
		io.ReadFull(r, l)
		io.CopyN(io.Discard, r, int64(l[0])+2)
	case 0x04: // IPv6
		io.CopyN(io.Discard, r, 16+2)
	}
	reply := []byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
	conn.Write(reply)
}

func runHandshake(t *testing.T, ln net.Listener, user, pass string) error {
	t.Helper()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	return socks5Handshake(conn, user, pass, "example.com:443")
}

func TestSocks5HandshakeWithAuthSucceeds(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go fakeSocks5(t, ln, "auth", "alice", "s3cret")
	if err := runHandshake(t, ln, "alice", "s3cret"); err != nil {
		t.Fatalf("credentialed handshake failed: %v", err)
	}
}

func TestSocks5HandshakeRejectsNoAuthDowngrade(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go fakeSocks5(t, ln, "downgrade", "alice", "s3cret")
	err = runHandshake(t, ln, "alice", "s3cret")
	if err == nil {
		t.Fatal("handshake must refuse a server that downgrades to no-auth")
	}
}

func TestSocks5HandshakeRejectsWrongCredentials(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go fakeSocks5(t, ln, "auth", "alice", "right")
	if err := runHandshake(t, ln, "alice", "wrong"); err == nil {
		t.Fatal("handshake should fail with wrong password")
	}
}

func TestSocks5HandshakeNoAuthWithoutCredentials(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go fakeSocks5(t, ln, "noauth", "", "")
	if err := runHandshake(t, ln, "", ""); err != nil {
		t.Fatalf("anonymous handshake failed: %v", err)
	}
}

func TestSocks5HandshakeConnectFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, _ := ln.Accept()
		if conn == nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		head := make([]byte, 2)
		io.ReadFull(r, head)
		io.CopyN(io.Discard, r, int64(head[1]))
		conn.Write([]byte{0x05, 0x00})
		req := make([]byte, 4)
		io.ReadFull(r, req)
		l := make([]byte, 1)
		io.ReadFull(r, l)
		io.CopyN(io.Discard, r, int64(l[0])+2)
		// general server failure
		conn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	}()
	if err := runHandshake(t, ln, "", ""); err == nil {
		t.Fatal("handshake should report CONNECT failure code")
	}
}

func TestSocks5GreetingOffersOnlyUserpassWhenCredentialsSet(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	greet := make(chan []byte, 1)
	go func() {
		conn, _ := ln.Accept()
		if conn == nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		head := make([]byte, 2)
		if _, err := io.ReadFull(r, head); err != nil {
			return
		}
		methods := make([]byte, head[1])
		io.ReadFull(r, methods)
		greet <- methods
	}()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	socks5Handshake(conn, "u", "p", "example.com:80") // outcome irrelevant
	select {
	case methods := <-greet:
		if len(methods) != 1 || methods[0] != 0x02 {
			t.Fatalf("offered methods = %v, want [0x02] only", methods)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no greeting received")
	}
}
