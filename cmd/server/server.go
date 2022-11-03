package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"github.com/cloudflare/tableflip"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	var (
		listenAddr = flag.String("listen", "localhost:8080", "`Address` to listen on")
		pidFile    = flag.String("pid-file", "main.pid", "`Path` to pid file")
	)

	flag.Parse()

	log.SetPrefix(fmt.Sprintf("%d ", os.Getpid()))

	upg, _ := tableflip.New(tableflip.Options{
		PIDFile: *pidFile,
	})
	defer upg.Stop()

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGHUP)
		for range sig {
			err := upg.Upgrade()
			if err != nil {
				log.Println("Upgrade failed:", err)
			}
		}
	}()

	ln, err := upg.Fds.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalln("Can't listen:", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	tcpServe(ctx, ln, upg, wg)

	log.Println("upg Ready")
	if err := upg.Ready(); err != nil {
		panic(err)
	}
	log.Println("upg Exit Before")
	<-upg.Exit()

	log.Println("cancel")
	cancel()
	wg.Wait()
	log.Println("upg Exit")

}

func tcpServe(ctx context.Context, ln net.Listener, upg *tableflip.Upgrader, wg sync.WaitGroup) {

	conn, err := upg.Conn("tcp", "")
	if err != nil || conn == nil {
		log.Println("get conn from upg error", err)
	} else {
		log.Printf("get conn from parent OK, serving %s", conn.RemoteAddr().String())
		go connServe(ctx, conn, upg, wg)
	}

	go func() {
		wg.Add(1)
		defer wg.Done()
		defer ln.Close()

		log.Printf("listening on %s", ln.Addr())

		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}

			go connServe(ctx, c, upg, wg)
		}
	}()

}

func connServe(ctx context.Context, c net.Conn, upg *tableflip.Upgrader, wg sync.WaitGroup) {
	log.Printf("accept and serving conn %s", c.RemoteAddr().String())
	upg.Fds.AddConn("tcp", "", c.(tableflip.Conn))
	wg.Add(1)
	exit := false
	for !exit {
		select {
		case <-ctx.Done():
			exit = true
		default:
			line, _, err := bufio.NewReader(c).ReadLine()
			if err != nil {
				log.Printf("Failure to read:%s", err.Error())
				return
			}
			_, err = c.Write([]byte("echo " + string(line) + "\n"))
			if err != nil {
				log.Printf("Failure to write: %s", err.Error())
				return
			}
		}
	}
	c.Close()
	wg.Done()
}
