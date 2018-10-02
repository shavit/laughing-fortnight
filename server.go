package laughing_fortnight

import (
  "errors"
  "io"
  "log"
  "net"
  "os"
  "sync"
)

// server holds the TCP server
type server struct {
  ln net.Listener // Network listener
  done chan error
  conns map[uint32]*edge
  *sync.RWMutex
}

// edge represents a TCP connection to the server
type edge struct {
  conn *io.ReadWriteCloser
  msg chan []byte // Simple message that can be extended with different headers
}

// NewServer initialize a new empty server
func NewServer() (s *server) {
  return &server{
    conns: make(map[uint32]*edge),
    RWMutex: new(sync.RWMutex),
  }
}

// Start starts the server to accept incoming connections, announce and
//  publish messages.
func (s *server) Start() (err error){
  var conn net.Conn
  var addr string = "127.0.0.1:8888"

  s.ln, err = net.Listen("tcp", addr)
  if err != nil {
    return errors.New("Error listening to " + addr)
  }

  // Listen to the exit signal
  go func() {
    // Block and wait for a signal to exit the program
    s.Close(<-s.done) // It will wait for the channel before executing
    log.Println("\n\nSending exit notificaiton to all clients\n")
  }()

  log.Println("Listening on", addr)
  for {
    conn, err = s.ln.Accept()
    if err != nil {
      log.Println(err)
      return errors.New("Error listening on port 8888")
    }

    // TODO: Handle the connection
    log.Println("New connection", conn)
  }

  return err
}

// Close sends a signal to the server, to exit gracefully.
//  The server can announce to the peers that it going to shutdown in case
//  that a custom message struct is delivered.
func (s *server) Close(err error) {
  log.Println("Closing the server.\n", err)

  // Announce closing to all the clients here
  // ..

  // Close the network connection
  err = s.ln.Close()
  log.Println("Closing the network connection.\n", err)

  // Exit the program
  os.Exit(0)
}
