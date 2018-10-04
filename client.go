package laughing_fortnight

import (
  "bufio"
  "fmt"
  "io"
  "log"
  "net"
  "os"
  "os/signal"
  "syscall"
  "time"
)

type ChatClient interface {
  Dial(network, address string) (err error)
  Close()
  handleInput()
  echoServer(done chan struct{})
}

type chatClient struct {
  conn net.Conn
}

func NewChatClient() ChatClient {
  return new(chatClient)
}

func (client *chatClient) Dial(network, address string) (err error){
  client.conn, err = net.Dial(network, address)
  return err
}

func (client *chatClient) Close(){
  client.conn.Close()
}

func (client *chatClient) handleInput(){
  var scanner *bufio.Scanner = bufio.NewScanner(os.Stdin)
  for scanner.Scan(){
    io.WriteString(client.conn, scanner.Text())
  }
}

func (client *chatClient) echoServer(done chan struct{}){
  for {
    buf := make([]byte, 12000)
    n, err := client.conn.Read(buf)
    if err != nil {
      log.Println("Connection closed by the server.")
      break
    }

    fmt.Println("[SERVER] " + string(buf[0:n]))
    print("> ")
    buf = nil
  }

  done <- struct{}{}
}


func StartClient(retry uint8){
  var err error
  var ch chan os.Signal = make(chan os.Signal)
  var client ChatClient = NewChatClient()
  var done chan struct{} = make(chan struct{})

  if err = client.Dial("tcp", "127.0.0.1:8888"); err != nil {
    log.Println(err)
    retryConnection(retry)
    return
  } else {
    retry = 3
  }
  defer client.Close()

  go func() {
    signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
    <-ch
    log.Println("\n\nDisconnecting from the server\n")
    os.Exit(0)
  }()

  go client.handleInput()
  go client.echoServer(done)
  <-done

  // Try to reconnect to a new server
  retryConnection(retry)
}

func retryConnection(retry uint8) {
  if retry == 0 {
    return
  }
  time.Sleep(time.Duration(int64(9/retry))  * time.Second)
  StartClient(retry - 1)
}
