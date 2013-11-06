package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func serveOne(conn net.Conn, data chan<- []byte) {
	b := make([]byte, 32)
	n, err := conn.Read(b)
	check(err)

	s := string(b[:n-1])
	log.Printf("Connection from %q", s)
	pid, err := strconv.ParseUint(s, 10, 64)
	check(err)

	proc := fmt.Sprintf("/proc/%v/", pid)

	io_content, err := ioutil.ReadFile(proc + "/io")
	if err != nil {
		panic(err)
	}

	status_content, err := ioutil.ReadFile(proc + "/status")
	if err != nil {
		panic(err)
	}

	err = conn.Close()
	check(err)

	data <- append(status_content, io_content...)
}

func writelog(data <-chan []byte, done <-chan struct{}) {
	defer println("Written log")

	fd, err := os.Create("paccountant.log")
	check(err)
	defer fd.Close()

	buf := &bytes.Buffer{}
	var datum []byte

	for {
		select {
		case datum = <-data:
		case <-done:
			log.Println("Recv <-done")
			return
		}

		n, err := buf.Write(datum)
		check(err)
		if n != len(datum) {
			check(fmt.Errorf("Unexpected number of bytes to buffer.. %v != %v",
				n, len(datum)))
		}

		_, err = io.Copy(fd, buf)
		check(err)

		fd.Write([]byte{0})
	}
}

func main() {
	listener, err := net.Listen("tcp4", "localhost:7117")
	check(err)
	defer listener.Close()

	log := make(chan []byte)
	done := make(chan struct{})

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()
		writelog(log, done)
	}()

	go func() {
		for {
			conn, err := listener.Accept()
			check(err)

			go serveOne(conn, log)
		}
	}()

	s := make(chan os.Signal)
	signal.Notify(s)
	<-s

	close(done)
}
