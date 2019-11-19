#!/bin/bash

echo "Content-type: text/html"
echo ""

if [ "$REQUEST_METHOD" != "POST" ]; then
  echo 'Post only'
  exit 0
fi

WORKER=$(cat - | sed -n 's/worker=//p')

if [ -z "$WORKER" ]; then
  echo 'Need worker=<hostname> in body'
  exit 0
fi

echo docker node update --label-add TGRole=worker $WORKER
docker node update --label-add TGRole=worker $WORKER

exit 0
~
