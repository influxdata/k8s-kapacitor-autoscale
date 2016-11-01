#!/bin/bash

# Slowly ramp up traffic to an HTTP endpoint.
# Once a maximum QPS is reached ramp back down to 0.
#
# Requires hey https://github.com/rakyll/hey to be installed.
#
# Usage: ./ramp.sh <URL> [maximum qps]
#
# Default maximum is 5000.
#
# Examples:
#   ./ramp.sh http://localhost:8888 # uses default max of 5000
#   ./ramp.sh http://localhost:8888 1000

URL=$1
max=${2-5000} # maximum QPS, when this value is reached the script will begin to ramp back down to 0

# Feel free to tweak these values as well

d=5 # 5s between ramp ups
c=1 # initial concurrency, i.e number of workers
q=10 # qps per worker
step=1 # number of new workers to add per step

qps=$(($q*$c))
while true
do
    qps=$(($q*$c))
    if [ $qps -le 0 ]
    then
        break
    fi
    echo $(date) "QPS $qps"

    n=$(($d * $q * $c))
    # Run hey with the current setting, it should terminate after $d seconds have passed
    hey -c $c -q $q -n $n $URL

    if [ $qps -ge $max ]
    then
        # Go back down
        step=$((-$step))
    fi

    c=$(($c+$step))
done
