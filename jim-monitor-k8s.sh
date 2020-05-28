#! /bin/bash

if [ -z "$1" ]; then
  echo "Need to specific instance number"
  exit 1
fi

while true; do
  CHECK=$(kubectl get pods | grep ^tg- | awk '{print $1}' | perl -pe 's/^tg-(.*)-([^-]+)-single-\d+$/\1 \2/' | sort | uniq | wc -l)

  if [ "$CHECK" -ne "1" ]; then
    echo "Too many or two few tg deploys: $CHECK"
  else
    BASE=$(kubectl get pods | grep ^tg- | awk '{print $1}' | perl -pe 's/^tg-(.*)-([^-]+)-single-\d+$/\1/' | sort | uniq)
    ID=$(kubectl get pods | grep ^tg- | awk '{print $1}' | perl -pe 's/^tg-(.*)-([^-]+)-single-\d+$/\2/' | sort | uniq)
    echo $BASE
    echo $ID

    kubectl exec -it tg-$BASE-$ID-single-$1 -- monitor.sh
  fi
  sleep 5
done

