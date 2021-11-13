#!/bin/bash

MANAGER_GO_ENTRY_POINT=cmd/manager/main.go
GATEWAY_GO_ENTRY_POINT=cmd/gateway/main.go

DOCKER_COMPOSE=docker-compose
TEST_DOCKER_COMPOSE_FILE=./build/docker-compose.yml
TEST_DOCKER_COMPOSE_UP_D="${DOCKER_COMPOSE} -f ${TEST_DOCKER_COMPOSE_FILE} up -d"
TEST_DOCKER_COMPOSE_DOWN="${DOCKER_COMPOSE} -f ${TEST_DOCKER_COMPOSE_FILE} down"

managerHost=localhost
managerPort=1883
gatewayHost=localhost
gatewayPort=1884

PID_FILE=".pid"
level=info
env=production
caller= 

if [ -f ${PID_FILE} ]; then
    cat ${PID_FILE} | xargs -I{} kill {}
    ${TEST_DOCKER_COMPOSE_DOWN}
    rm ${PID_FILE}
elif [ "${1}" = "run" ]; then
    # MQTT ブローカの起動
    ${TEST_DOCKER_COMPOSE_DOWN}
    ${TEST_DOCKER_COMPOSE_UP_D}

    # manager の起動
    go run cmd/manager/main.go -level ${level} \
                               -env ${env} ${caller} \
                               -host ${managerHost} \
                               -port ${managerPort} > manager00.log &
    echo $! > ${PID_FILE}

    # gateway の起動
    go run cmd/gateway/main.go -level ${level} \
                               -env ${env} ${caller} \
                               -managerHost ${managerHost} \
                               -managerPort ${managerPort} \
                               -gatewayHost ${gatewayHost} \
                               -gatewayPort ${gatewayPort} > gateway00.log &
    echo $! >> ${PID_FILE}

    # manager と gateway を停止させるために、プロセスIDを保持する
    # NOTE: sleep を入れないと、プロセスIDをファイルに記録することが出来なかった
    sleep 1
    ps | grep main | grep -oE "^[0-9]+" >> ${PID_FILE}

    # manager へ分散ブローカを設定する
    mosquitto_pub -h localhost -p 1883 -t "/api/tool/distributedbroker/add" -m '{"topic":"/","broker_info":{"host":"localhost","port":1893}}'
    sleep 1
    mosquitto_pub -h localhost -p 1883 -t "/api/tool/distributedbroker/add" -m '{"topic":"/1","broker_info":{"host":"localhost","port":1894}}'

    # manager へ gateway の担当エリアを設定する
    sleep 1
    mosquitto_pub -h localhost -p 1883 -t "/api/tool/gatewaybroker/set" -m '{"topic":"/","broker_info":{"host":"localhost","port":1884}}'
    sleep 1
    mosquitto_pub -h localhost -p 1883 -t "/api/tool/gatewaybroker/set" -m '{"topic":"/0","broker_info":{"host":"localhost","port":1884}}'
fi
