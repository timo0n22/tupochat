package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
)

func main() {

	conn, _ := net.Dial("tcp", "localhost:9999")
	var text string
	reader := bufio.NewReader(os.Stdin)

	go func() {
		reader := bufio.NewReader(conn)
		for {
			msg, err := reader.ReadString('\n')
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
