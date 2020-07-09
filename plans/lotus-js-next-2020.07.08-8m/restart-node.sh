#! /bin/sh

PID=$(ps -ef | grep '\/lotus\/lotus ' | awk '{print $2}')
echo $PID

if [ -n "$PID" ]; then
	kill $PID
fi

echo "\n>>> Restarted $(date)\n" >> /outputs/node.out

echo Restarting...
nohup /lotus/lotus daemon >> /outputs/node.out 2>&1 &

sleep 2
tail -40 /outputs/node.out


