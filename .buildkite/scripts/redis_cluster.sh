#!/bin/sh
# shellcheck disable=SC1117,SC2103,SC2086,SC2039

# Settings
PORT=30000
TIMEOUT=2000
NODES=6
REPLICAS=1

# Computed vars
ENDPORT=$((PORT+NODES))

if [ "$1" == "start" ]
then
    while [ $((PORT < ENDPORT)) != "0" ]; do
        PORT=$((PORT+1))
        echo "Starting $PORT"
        redis-server --port $PORT --cluster-enabled yes --cluster-config-file nodes-${PORT}.conf --cluster-node-timeout $TIMEOUT --appendonly yes --appendfilename appendonly-${PORT}.aof --dbfilename dump-${PORT}.rdb --logfile ${PORT}.log --daemonize yes
    done

    # if running docker as a daemon container we still want a process
    # to keep running so that the container does not die
    while true; do
        echo "Redis cluster running"
        sleep 10
    done

    exit 0
fi

if [ "$1" == "create" ]
then
    HOSTS=""
    while [ $((PORT < ENDPORT)) != "0" ]; do
        PORT=$((PORT+1))
        HOSTS="$HOSTS 127.0.0.1:$PORT"
        redis-cli -p $PORT --eval /app/mqueue/redis/redis.lua
    done

    redis-cli --cluster create $HOSTS --cluster-replicas $REPLICAS
    exit 0
fi

if [ "$1" == "stop" ]
then
    while [ $((PORT < ENDPORT)) != "0" ]; do
        PORT=$((PORT+1))
        echo "Stopping $PORT"
        redis-cli -p $PORT shutdown nosave
    done
    exit 0
fi

if [ "$1" == "watch" ]
then
    PORT=$((PORT+1))
    while true; do
        clear
        date
        redis-cli -p $PORT cluster nodes | head -30
        sleep 1
    done
    exit 0
fi

if [ "$1" == "tail" ]
then
    INSTANCE=$2
    PORT=$((PORT+INSTANCE))
    tail -f ${PORT}.log
    exit 0
fi

if [ "$1" == "call" ]
then
    while [ $((PORT < ENDPORT)) != "0" ]; do
        PORT=$((PORT+1))
        redis-cli -p $PORT $2 $3 $4 $5 $6 $7 $8 $9
    done
    exit 0
fi

if [ "$1" == "clean" ]
then
    rm -rf ./*.log
    rm -rf ./appendonly*.aof
    rm -rf ./dump*.rdb
    rm -rf ./nodes*.conf
    exit 0
fi

if [ "$1" == "clean-logs" ]
then
    rm -rf ./*.log
    exit 0
fi

echo "Usage: $0 [start|create|stop|watch|tail|clean]"
echo "start       -- Launch Redis Cluster instances."
echo "create      -- Create a cluster using redis-cli --cluster create."
echo "stop        -- Stop Redis Cluster instances."
echo "watch       -- Show CLUSTER NODES output (first 30 lines) of first node."
echo "tail <id>   -- Run tail -f of instance at base port + ID."
echo "clean       -- Remove all instances data, logs, configs."
echo "clean-logs  -- Remove just instances logs."
