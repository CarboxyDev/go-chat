package main

import (
	"fmt"
	"net"

	"github.com/fatih/color"
)


func main() {
	const PORT = "8080"
	server, err := net.Listen("tcp", ":" + PORT);

	if err != nil {
		panic(err);
	}

	defer server.Close();
	color.Green("[+] Server has been started on port %s", PORT );

	for {
		conn, err := server.Accept();

		if err != nil {
			color.Red("[x] Unable to accept connection. ")
			color.Red("[x] Error: " + err.Error())
			continue;
		}

		go connectionHandler(conn);
	}

}


func connectionHandler(conn net.Conn) {
	defer conn.Close();

	buffer := make([]byte, 1024 * 100); // 100 KB buffer for incoming messages

	for {
		n, err := conn.Read(buffer);
		if err != nil {
			color.Blue("[-] Client disconnected");
			return;
		}

		data := buffer[:n]
		fmt.Printf("[+] Received data: %s\n", data);

		conn.Write([]byte("Echoed back: " + string(data)))

	}

}