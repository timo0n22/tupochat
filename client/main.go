package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
)

func main() {

	var text string
	reader := bufio.NewReader(os.Stdin)
	conn, _ := net.Dial("tcp", "localhost:9999")

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
		fmt.Print("\033[1A")
		fmt.Print("\033[2K")
		if text == "/exit\n" {
			break
		}
		conn.Write([]byte(text))
	}
	fmt.Println("Exiting...")
}
