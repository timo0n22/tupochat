package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func main() {
	conn, _ := net.Dial("tcp", "localhost:9999")
	var text string
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("message: ")
		text, _ = reader.ReadString('\n')
		if text == "/exit" {
			break
		}
		conn.Write([]byte(text))
	}
}
