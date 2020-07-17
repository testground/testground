#! /bin/sh

PID=$(ps -ef | grep '\/lotus\/lotus-miner' | awk '{print $2}')
echo $PID

if [ -n "$PID" ]; then
	kill $PID
fi

echo "\n>>> Restarted $(date)\n" >> /outputs/miner.out

echo Restarting...
nohup /lotus/lotus-miner run >> /outputs/miner.out 2>&1 &

sleep 2
tail -40 /outputs/miner.out


