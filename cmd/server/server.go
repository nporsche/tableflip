package main

import (
	"bufio"
	"context"
	"flag"
	"github.com/golang/glog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"tableflip.test.com/tableflip"
)

func main() {
	var (
		network    = flag.String("net", "unix", "unix, tcp")
		listenAddr = flag.String("listen", "localhost:8080", "`Address` to listen on")
		pidFile    = flag.String("pid-file", "main.pid", "`Path` to pid file")
	)

	flag.Parse()
	defer glog.Flush()

	upg, _ := tableflip.New(tableflip.Options{
		PIDFile: *pidFile,
	})

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGHUP, syscall.SIGTERM)
		for s := range sig {
			if s == syscall.SIGHUP {
				err := upg.Upgrade()
				if err != nil {
					glog.Infoln("Upgrade failed:", err)
				}
			}
			if s == syscall.SIGTERM {
				upg.Stop()
				glog.Infoln("term signal, stopping")
			}
		}
	}()

	//ln, err := upg.Fds.ListenWithCallback(*network, *listenAddr, createNewListen) //2.46
	ln, err := createNewListen(*network, *listenAddr) //2.45
	if err != nil {
		glog.Fatalln("Can't listen:", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	tcpServe(ctx, ln, upg, wg)

	glog.Infoln("upg Ready")
	if err := upg.Ready(); err != nil {
		panic(err)
	}

	<-upg.Exit()
	glog.Infof("close listener %s:%s", *network, *listenAddr)
	ln.Close()

	cancel()
	wg.Wait()
	glog.Infoln("upg Exit")

}

func createNewListen(network, addr string) (net.Listener, error) {
	glog.Infof("create new listener %s:%s", network, addr)

	if network == "unix" {
		if _, err := os.Stat(filepath.Dir(addr)); err != nil {
			os.Mkdir(filepath.Dir(addr), 0777)
		}
		if _, err := os.Stat(addr); err == nil {
			glog.Infof("remove %s", addr)
			os.Remove(addr)
		}
	}

	return net.Listen(network, addr)
}

func tcpServe(ctx context.Context, ln net.Listener, upg *tableflip.Upgrader, wg sync.WaitGroup) {

	conn, err := upg.Conn("tcp", "")
	if err != nil || conn == nil {
		glog.Infoln("get conn from upg error", err)
	} else {
		glog.Infof("get conn from parent OK, serving %s", conn.RemoteAddr().String())
		go connServe(ctx, conn, upg, wg)
	}

	go func() {
		wg.Add(1)
		defer wg.Done()
		defer ln.Close()

		glog.Infof("listening on %s", ln.Addr())

		for {
			c, err := ln.Accept()
			if err != nil {
				glog.Errorf("accept error %s", err)
				return
			}

			go connServe(ctx, c, upg, wg)
		}
	}()

}

func connServe(ctx context.Context, c net.Conn, upg *tableflip.Upgrader, wg sync.WaitGroup) {
	glog.Infof("accept and serving conn %s", c.RemoteAddr().String())
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
				glog.Infof("Failure to read:%s", err.Error())
				return
			}
			_, err = c.Write([]byte("echo " + string(line) + "\n"))
			if err != nil {
				glog.Infof("Failure to write: %s", err.Error())
				return
			}
		}
	}
	c.Close()
	wg.Done()
}
