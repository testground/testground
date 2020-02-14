#! /bin/bash

docker ps | grep tg- | awk '{print $1}' | xargs docker stop ; docker container prune -f

