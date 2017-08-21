package fullstory_test

import (
	"fmt"
	"log"

	"github.com/nishanths/fullstory"
)

func Example_usage() {
	client := fullstory.NewClient("API token")

	s, err := client.Sessions(15, "foo", "hikingfan@gmail.com")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(s)
}
