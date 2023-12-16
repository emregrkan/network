package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

type Server interface {
	Run()
}

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

func (tr *tcpReader) readConnection(byteChan chan []byte, errChan chan error) {
	scanner := bufio.NewScanner(tr)
	for scanner.Scan() {
		byteChan <- scanner.Bytes()
	}
	if err := scanner.Err(); err != nil {
		errChan <- err
		return
	}
	// Explicit EOF since scanner.Err() filters it
	errChan <- io.EOF
}

func (server *TCPServer) processRequest(data []byte) {
	var req string
	var timestamp, val int

	parsed, err := fmt.Sscanf(string(data), "%s %d:%d", &req, &timestamp, &val)
	if parsed < 3 || err != nil {
		if _, err := fmt.Fprint(server.tempConn, 400); err != nil {
			log.Println("Error sending response")
		}
		return
	}

	if _, err := fmt.Fprint(server.tempConn, 200); err != nil {
		log.Println("Error sending response")
	}

	server.buffer = append(server.buffer, fmt.Sprintf("%d:%d", timestamp, val))

	if len(server.buffer) == 10 {
		// TODO: Send the buffer to the server
		log.Println(server.buffer)
		clear(server.buffer)
	}
}

func (server *TCPServer) Run() {
	// Broadcast port
	ln, err := net.Listen("tcp", server.Port)
	if err != nil {
		log.Fatal("Failed to create TCP listener: ", err)
	}

	// Close the listener
	defer ln.Close()

	// Accept connection attempts
	// Note: TCP Server accepts a single active connection
	for server.tempConn == nil {
		server.tempConn, err = ln.Accept()
		if err != nil {
			log.Println("Failed to connect to the client: ", err)
		}

		tr := &tcpReader{Conn: server.tempConn}

		byteChan := make(chan []byte)
		errChan := make(chan error)

		go tr.readConnection(byteChan, errChan)

		for server.tempConn != nil {
			select {
			// Received request successfully
			case data := <-byteChan:
				server.processRequest(data)
			case err := <-errChan:
				// TODO: Notify the server that TEMP Sensor down
				// Check if the client timed out
				if errors.Is(err, io.EOF) {
					log.Println("Client timed out, closing connection")
					// Closing the connection
					if err = server.tempConn.Close(); err != nil {
						log.Fatal(err)
					}
					// Set the connection to nil so can server accept a new connection
					server.tempConn = nil
				} else {
					// TODO: might send status 500
					log.Println(err)
					return
				}
			}
		}
	}
}
