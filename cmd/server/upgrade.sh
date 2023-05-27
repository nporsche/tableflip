#!/bin/sh

go build server.go

FILE=./main.pid
if test -f "$FILE"; then
  oldPID=`cat $FILE`
  echo $oldPID
  kill -s HUP $oldPID
else
  ./server -net unix -listen ./unix.sock -logtostderr
fi