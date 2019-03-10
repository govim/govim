package main

import (
	"io"
	"net"
	"os"
)

func runAsServer() {
	ln, err := net.Listen("tcp", *fAddr)
	if err != nil {
		fatalf("failed to listen on %v: %v", *fAddr, err)
	}
	// at this point, if we were able to bind we can safely assume we are the
	// singleton instance and grab a reference to our log file

	// TODO make this a tempfile; keeping as a known file to ease debugging
	path := "/tmp/govim_server.log"
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_SYNC, 0644)
	if err != nil {
		fatalf("failed to open %v: %v", path, err)
	}
	dl := io.MultiWriter(f, os.Stderr)

	debugf(dl, "===========================================")
	debugf(dl, "running as a server with debug = %v", *fDebug)
	debugf(dl, "listening on address %v", *fAddr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			debugf(dl, "failed to accept connection: %v", err)
		}
		h := newHandler(dl, conn)
		go h.handle()
	}
}
