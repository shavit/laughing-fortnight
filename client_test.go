package laughing_fortnight

import (
  "testing"
)

func TestCreateNewChatClient(t *testing.T){
  var client ChatClient = NewChatClient()
  var cClient *chatClient
  var ok bool

  cClient, ok = client.(*chatClient)
  if ok != true {
    t.Error("Error creating a client. ChatCilent not implemented")
  }

  if cClient.conn != nil {
    t.Error("Connection should be empty")
  }
}
