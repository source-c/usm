// Copyright 2020 Google LLC
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd

// Code in this file has been adapted from https://github.com/FiloSottile/yubikey-agent/blob/v0.1.6/main.go#L77
// released under the above license
package agent

import (
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"golang.org/x/term"
)

func Run(a *Agent, socketPath string) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		log.Println("Warning: usm-agent is meant to run as a background daemon.")
		log.Println("Running multiple instances is likely to lead to conflicts.")
		log.Println("Consider using the launchd or systemd services.")
	}
	c := make(chan os.Signal, 1)

	signal.Notify(c, syscall.SIGHUP)
	go func() {
		for range c {
			a.Close()
		}
	}()

	if runtime.GOOS != "windows" {
		// ATTN: create socket directory with restrictive permissions (owner-only)
		if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
			log.Fatalln("Failed to create UNIX socket folder:", err)
		}
		// Remove stale socket only if it exists and is owned by the current user
		if info, err := os.Lstat(socketPath); err == nil {
			if info.Mode()&os.ModeSocket != 0 {
				os.Remove(socketPath)
			} else {
				log.Fatalln("Socket path exists but is not a socket:", socketPath)
			}
		}
	}

	l, err := listen(socketPath)
	if err != nil {
		log.Fatalln("Failed to listen on socket:", err)
	}
	defer l.Close()

	for {
		c, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				log.Println("Listener closed, stopping agent")
				return
			}
			var netErr *net.OpError
			if errors.As(err, &netErr) && netErr.Temporary() {
				log.Println("Temporary Accept error, sleeping 1s:", err)
				time.Sleep(1 * time.Second)
				continue
			}
			log.Println("Failed to accept connections:", err)
			return
		}
		go a.serveConn(c)
	}
}
