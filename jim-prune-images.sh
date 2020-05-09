#! /bin/bash

docker images | awk '{print $1 ":" $2}' | grep '^[0-9]' | xargs docker rmi
#docker images | awk '{print $1 ":" $2}'
