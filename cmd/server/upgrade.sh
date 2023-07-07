#!/bin/bash
set -x
FILE=./main.pid

if [[ $1 == "c" ]]; then
  echo "clean start"
  oldPID=`cat $FILE`
  echo $oldPID
  kill $oldPID
  rm unix.sock
  rm $FILE
  echo "clean done"
  exit 0
fi

go build server.go

if test -f "$FILE"; then
  oldPID=`cat $FILE`
  echo $oldPID
  kill -s HUP $oldPID
else
  ./server -net unix -listen ./unix.sock -logtostderr
fi

