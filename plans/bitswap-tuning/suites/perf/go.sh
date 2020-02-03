#!/bin/bash -x

# time ./plans/bitswap-tuning/suites/suite.sh ./results/100ms-40MB 40
# time ./plans/bitswap-tuning/suites/suite.sh ./results/100ms-20MB 20
# time ./plans/bitswap-tuning/suites/suite.sh ./results/100ms-10MB 10

# node ./plans/bitswap-tuning/suites/chart.js -d ./results/100ms-40MB/ -m time_to_fetch -l 100 -b 40 \
#   -xlabel 'File size (MB)' -ylabel 'Time to fetch (s)' -xscale '9.53674316e-7' -yscale '1e-9' \
#   && gnuplot ./results/100ms-40MB/time_to_fetch.plot > ./results/100ms-40MB/100ms-40MB.svg

# node ./plans/bitswap-tuning/suites/chart.js -d ./results/100ms-20MB/ -m time_to_fetch -l 100 -b 20 \
#   -xlabel 'File size (MB)' -ylabel 'Time to fetch (s)' -xscale '9.53674316e-7' -yscale '1e-9' \
#   && gnuplot ./results/100ms-20MB/time_to_fetch.plot > ./results/100ms-20MB/100ms-20MB.svg

# node ./plans/bitswap-tuning/suites/chart.js -d ./results/100ms-10MB/ -m time_to_fetch -l 100 -b 10 \
#   -xlabel 'File size (MB)' -ylabel 'Time to fetch (s)' -xscale '9.53674316e-7' -yscale '1e-9' \
#   && gnuplot ./results/100ms-10MB/time_to_fetch.plot > ./results/100ms-10MB/100ms-10MB.svg
