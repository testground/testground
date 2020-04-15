#! /bin/bash

CHECK=$(kubectl get pods | grep ^tg- | awk '{print $1}' | perl -pe 's/^tg-(.*)-([^-]+)-single-\d+$/\1 \2/' | sort | uniq | wc -l)

if [ "$CHECK" -ne "1" ]; then
	echo "Too many or two few tg deploys: $CHECK"
	exit 1
fi
BASE=$(kubectl get pods | grep ^tg- | awk '{print $1}' | perl -pe 's/^tg-(.*)-([^-]+)-single-\d+$/\1/' | sort | uniq)
ID=$(kubectl get pods | grep ^tg- | awk '{print $1}' | perl -pe 's/^tg-(.*)-([^-]+)-single-\d+$/\2/' | sort | uniq)
echo $BASE
echo $ID

if [ -z "$1" ]; then
	echo "Need to specific instance number"
	exit 1
fi

exec kubectl exec -it tg-$BASE-$ID-single-$1 -- /bin/bash

