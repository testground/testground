#! /bin/bash

for h in `seq $1 $2`; do
	echo $h
	BLOCKS=`./blocks-at-height.sh $h`
	for b in $BLOCKS; do
		echo Block: $b
		PARENT=$(lotus chain get `lotus chain getblock $b | jq -r '.Messages["/"]'` | jq -r '.[]["/"]' | grep -v bafy2bzaceaa43et73tgxsoh2xizd4mxhbrcfig4kqp25zfa5scdgkzppllyuu)
		for p in $PARENT; do
			echo "  Parent: $p"
			MESSAGES=$(lotus chain get $p | jq -r '.[2][2][]["/"]')
			for m in $MESSAGES; do
				echo "    Message: $m"
				lotus chain getmessage $m | sed 's,^,    ,'
			done
		done
	done
done
