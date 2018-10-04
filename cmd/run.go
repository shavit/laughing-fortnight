package main

import (
  "flag"
  "os"
  "github.com/shavit/laughing_fortnight"
)

const (
  SERVER = "server"
  CLIENT = "client"
)

func main(){
  var fHost = flag.String("h", "127.0.0.1", "host address")
  var fPort = flag.Int("p", 8888, "port number")
  var fEndpoint = flag.String("endpoint", "127.0.0.1:2379", "etcd client endpoint")
  flag.Parse()

  if len(os.Args) <= 1 {
    printHelp()
    os.Exit(1)
  }

  switch os.Args[len(os.Args)-1] {
  case SERVER:
    laughing_fortnight.StartServer(*fHost, uint16(*fPort), *fEndpoint)
    break
  case CLIENT:
    laughing_fortnight.StartClient(12)
    break
  default:
    printHelp()
    os.Exit(1)
  }
}

func printHelp(){
  println(`
    Usage: run [OPTIONS] MODE

    Mode:
      server  - Start a TCP server on 127.0.0.1:8888
      client  - Connect to a TCP server on 127.0.0.1:8888

    Options:
      -p            - port number, defaults to 8888
      -h            - host address, defaults to 127.0.0.1
      -endpoint     - etcd endpoint, defaults to 127.0.0.1:2379
`)
}
