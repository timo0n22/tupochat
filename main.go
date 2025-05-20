package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

type Client struct {
	conn net.Conn
	id   int
}

var clients []Client
var id = 0

func distribute(from string, message string, exclude int) {
	msg := fmt.Sprintf("%s: %s", from, message)
	for _, client := range clients {
		if client.id != exclude {
			client.conn.Write([]byte(msg))
		}
	}
}

func handleConnection(c Client) {
	defer c.conn.Close()
	reader := bufio.NewReader(c.conn)
	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("client %d disconnected\n", c.id)
			return
		}
		if msg == "/exit\n" {
			fmt.Printf("client %d exit\n", c.id)
			return
		}
		fmt.Printf("client %d: %s", c.id, msg)
		distribute((fmt.Sprintf("client %d", c.id)), msg, c.id)
	}
}

func main() {
	ln, _ := net.Listen("tcp", ":9999")
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			msg, _ := reader.ReadString('\n')
			distribute("server", msg, 0)
		}
	}()
	for {
		conn, _ := ln.Accept()
		id++
		clients = append(clients, Client{conn, id})
		go handleConnection(Client{conn, id})
	}
}
