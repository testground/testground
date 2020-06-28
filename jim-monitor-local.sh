#! /bin/bash

CHECK=$(docker ps --format "{{.Names}}" | grep ^tg- | perl -pe 's/^tg-(.*)-([^-]+)-single-\d+$/\1 \2/' | sort | uniq | wc -l)

if [ "$CHECK" -ne "1" ]; then
	echo "Too many or too few tg deploys: $CHECK"
	exit 1
fi
BASE=$(docker ps --format "{{.Names}}" | grep ^tg- | perl -pe 's/^tg-(.*)-([^-]+)-single-\d+$/\1/' | sort | uniq)
ID=$(docker ps --format "{{.Names}}" | grep ^tg- | perl -pe 's/^tg-(.*)-([^-]+)-single-\d+$/\2/' | sort | uniq)
echo $BASE
echo $ID

if [ -z "$1" ]; then
	echo "Need to specific instance number"
	exit 1
fi

docker exec -it tg-$BASE-$ID-single-$1 monitor.sh

