package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"syscall"
	"time"
)

type Server interface {
	Run()
}

type UDPServer struct{}

type TCPServer struct {
	Port     string
	tempConn net.Conn
	buffer   []string
	// server connection
}

type tcpReader struct {
	net.Conn
}

func (tr *tcpReader) Read(p []byte) (n int, err error) {
	// Read bytes
	n, err = tr.Conn.Read(p)

	// Set timeout value after each message
	// Ignore the error check
	tr.Conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	// Check if client timed out
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		err = io.EOF
	}

	return
}

func (tr *tcpReader) readConnection(reqChan chan []byte, errChan chan error) {
	scanner := bufio.NewScanner(tr)
	for scanner.Scan() {
		reqChan <- scanner.Bytes()
	}
	if err := scanner.Err(); err != nil {
		errChan <- err
		return
	}
	// Explicit EOF since scanner.Err() filters it
	errChan <- io.EOF
}

func (srv *TCPServer) sendResponse(status int, req string) {
	// Use with valid connection
	if _, err := fmt.Fprint(srv.tempConn, status); err != nil {
		log.Println("Error sending response")
	}
	log.Println("Response status", status, "sent for request:", req)
}

func (srv *TCPServer) processRequest(req []byte) {
	reqString := string(req)
	var reqType string
	var timestamp, val int

	parsed, err := fmt.Sscanf(reqString, "%s %d:%d", &reqType, &timestamp, &val)
	if parsed < 3 || err != nil || reqType != "TEMP" {
		go srv.sendResponse(400, reqString)
		return
	}

	go srv.sendResponse(200, reqString)

	log.Println("Request:", reqString)

	// Critical
	srv.buffer = append(srv.buffer, string(req)[5:])
	if len(srv.buffer) == 10 {
		// TODO: Send the buffer to the server
		log.Println(srv.buffer)
		// Clear the slice, keep the allocated memory
		srv.buffer = srv.buffer[:0]
	}
}

func (srv *TCPServer) Run() {
	// Broadcast port
	ln, err := net.Listen("tcp", srv.Port)
	if err != nil {
		log.Fatal("Failed to create TCP listener:", err)
	}

	// Close the listener
	defer ln.Close()

	// Accept connection attempts
	// Note: TCP Server accepts a single active connection
	for srv.tempConn == nil {
		srv.tempConn, err = ln.Accept()
		if err != nil {
			log.Println("Failed to connect to the client:", err)
		}

		tr := &tcpReader{Conn: srv.tempConn}

		reqChan := make(chan []byte)
		errChan := make(chan error)

		go tr.readConnection(reqChan, errChan)

		for srv.tempConn != nil {
			select {
			// Received request successfully
			case req := <-reqChan:
				srv.processRequest(req)
			case err := <-errChan:
				// TODO: Notify the server that the TEMP Sensor down
				// Check if the client timed out or connection resetted
				if errors.Is(err, io.EOF) || errors.Is(err, syscall.ECONNRESET) {
					log.Println("Connection reset")
					// Closing the connection
					if err = srv.tempConn.Close(); err != nil {
						log.Fatal(err)
					}
					// Set the connection to nil so can server accept a new connection
					srv.tempConn = nil
				} else {
					// Possible unrecoverable error
					// TODO: might send status 500
					log.Println(err)
					// Shuts down the business
					return
				}
			}
		}
	}
}
