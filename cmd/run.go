package main

import (
  "os"
  "github.com/shavit/laughing_fortnight"
)

const (
  SERVER = "server"
  CLIENT = "client"
)

func main(){
  if len(os.Args) <= 1 {
    printHelp()
    os.Exit(1)
  }

  switch os.Args[1] {
  case SERVER:
    laughing_fortnight.StartServer()
    break
  case CLIENT:
    laughing_fortnight.StartClient()
    break
  default:
    printHelp()
    os.Exit(1)
  }
}

func printHelp(){
  println(`
    Usage: run MODE

    Mode:
      server  - Start a TCP server on 127.0.0.1:8888
      client  - Connect to a TCP server on 127.0.0.1:8888
`)
}
