package main

import (
	"github.com/fullstorydev/hauser/client"
	"github.com/fullstorydev/hauser/core"
)

// this is just a way to invoke the standard entry point of hauser in core
// with the standard client
func main() {
	conf:=core.MustGetConfig()
	core.Main(client.NewDefaultClient(conf))
}
