package laughing_fortnight

import (
  "errors"
  "io"
  "log"
  "net"
  "os"
  "sync"
  "time"
)

type Server interface {
  // Start starts the server to accept incoming connections, announce and
  //  publish messages.
  Start() error

  // addClient appends a connection to the server
  addClient(conn net.Conn)

  // removeClient removes a client from the connection list
  removeClient(conn net.Conn)

  // replyMessage echos the message to the sender
  replyMessage(client *edge, message []byte)

  // broadcastMessage broadcasts a message to all the clients
  broadcastMessage(client *edge, message []byte)

  // Close sends a signal to the server, to exit gracefully.
  //  The server can announce to the peers that it going to shutdown in case
  //  that a custom message struct is delivered.
  Close(err error)
}

// server holds the TCP server
type server struct {
  ln net.Listener // Network listener
  done chan error
  conns []*edge
  *sync.RWMutex
}

// edge represents a TCP connection to the server
type edge struct {
  id int
  conn net.Conn
  joinedAt int64
}

// StartServer starts a server
func StartServer(){
  var err error
  var srv = NewServer()
  if err = srv.Start(); err != nil {
    log.Fatal(err)
  }
}

// NewServer initialize a new empty server
func NewServer() (s Server) {
  return &server{
    conns: make([]*edge, 0),
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

    go s.addClient(conn)
  }

  return err
}

// addClient appends a connection to the server
func (s *server) addClient(conn net.Conn) {
  var newEdge *edge = &edge{
    id: len(s.conns)+1,
    conn: conn,
    joinedAt: time.Now().Unix(),
  }

  s.Lock()
  s.conns = append(s.conns, newEdge)
  s.Unlock()
  defer s.removeClient(conn)

  var err error
  var buf []byte

  L:
  for {
    buf = make([]byte, 12000)
    _, err = conn.Read(buf)
    switch err {
    case io.EOF:
      log.Println("User", newEdge.id, "/", len(s.conns), "has left")
      break L
    case nil:
      break
    default:
      log.Fatal(err)
    }

    s.replyMessage(newEdge, buf)
    s.broadcastMessage(newEdge, buf)
  }
}

// removeClient removes a client from the connection list
func (s *server) removeClient(conn net.Conn) {
  for i, edge_ := range s.conns {
    if (edge_.conn == conn){
      s.Lock()
      // Replace the last item with this edge
      s.conns[i] = s.conns[len(s.conns)-1]
      s.conns = s.conns[:len(s.conns)-1]
      s.Unlock()
    }
  }
}

// replyMessage echos the message to the sender
func (s *server) replyMessage(client *edge, message []byte) {
  client.conn.Write(message)
}

// broadcastMessage broadcasts a message to all the clients
func (s *server) broadcastMessage(client *edge, message []byte){
  for _, edge_ := range s.conns {
    if (edge_.conn == client.conn){
      // Optional - do not broadcast to the sender
      break
    }

    s.replyMessage(edge_, message)
  }
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
