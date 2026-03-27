package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

func main() {

	var text string
	reader := bufio.NewReader(os.Stdin)
	conn, _ := net.Dial("tcp", "89.44.86.79:5522")

	go func() {
		msgReader := bufio.NewReader(conn)
		for {
			msg, err := msgReader.ReadString('\n')
			if err != nil {
				log.Fatalf("disconnected from server")
			}
			msg = strings.TrimSuffix(msg, "\n")
			fmt.Println(msg)
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
