package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
)

func main() {

	fmt.Print("name:")
	var text string
	reader := bufio.NewReader(os.Stdin)
	name, _ := reader.ReadString('\n')
	conn, _ := net.Dial("tcp", "localhost:9999")
	conn.Write([]byte(name))

	go func() {
		msgReader := bufio.NewReader(conn)
		for {
			msg, err := msgReader.ReadString('\n')
			if err != nil {
				log.Fatalf("disconnected from server")
			}
			fmt.Print(msg)
		}
	}()
	
	for {
		text, _ = reader.ReadString('\n')
		if text == "/exit\n" {
			break
		}
		conn.Write([]byte(text))
	}
	fmt.Println("Exiting...")
}
