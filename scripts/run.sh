#!/bin/bash

MANAGER_GO_ENTRY_POINT=cmd/manager/main.go
GATEWAY_GO_ENTRY_POINT=cmd/gateway/main.go

DOCKER_COMPOSE=docker-compose
TEST_DOCKER_COMPOSE_FILE=./scripts/local/docker-compose.yml
TEST_DOCKER_COMPOSE_UP_D="${DOCKER_COMPOSE} -f ${TEST_DOCKER_COMPOSE_FILE} up -d"
TEST_DOCKER_COMPOSE_DOWN="${DOCKER_COMPOSE} -f ${TEST_DOCKER_COMPOSE_FILE} down"

managerHost=localhost
managerPort=1883
gateway00Host=localhost
gateway00Port=1884
gateway01Host=localhost
gateway01Port=1885

dmb00Host=localhost
dmb00Port=1893
dmb01Host=localhost
dmb01Port=1894

PID_FILE=".pid"
level=debug
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
    # sleep 1

    # gateway00 の起動
    go run cmd/gateway/main.go -level ${level} \
                               -env ${env} ${caller} \
                               -managerHost ${managerHost} \
                               -managerPort ${managerPort} \
                               -gatewayHost ${gateway00Host} \
                               -gatewayPort ${gateway00Port} > gateway00.log &
    echo $! >> ${PID_FILE}

    # gateway01 の起動
    go run cmd/gateway/main.go -level ${level} \
                               -env ${env} ${caller} \
                               -managerHost ${managerHost} \
                               -managerPort ${managerPort} \
                               -gatewayHost ${gateway01Host} \
                               -gatewayPort ${gateway01Port} > gateway01.log &
    echo $! >> ${PID_FILE}

    # dmb00 の起動
    go run cmd/dmb/main.go -level ${level} \
                           -env ${env} ${caller} \
                           -managerHost ${managerHost} \
                           -managerPort ${managerPort} \
                           -dmbHost ${dmb00Host} \
                           -dmbPort ${dmb00Port} \
                           -dmbTopic "/" > dmb00.log &
    echo $! >> ${PID_FILE}

    # dmb01 の起動
    go run cmd/dmb/main.go -level ${level} \
                           -env ${env} ${caller} \
                           -managerHost ${managerHost} \
                           -managerPort ${managerPort} \
                           -dmbHost ${dmb01Host} \
                           -dmbPort ${dmb01Port} \
                           -dmbTopic "/0" > dmb01.log &
    echo $! >> ${PID_FILE}

    # manager と gateway を停止させるために、プロセスIDを保持する
    # NOTE: sleep を入れないと、プロセスIDをファイルに記録することが出来なかった
    # sleep 2
    ps | grep main | grep -oE "^\s*[0-9]+" >> ${PID_FILE}

    # manager へ分散ブローカを設定する
    # mosquitto_pub -h localhost -p 1883 -t "/api/tool/distributedbroker/add" -m '{"topic":"/","broker_info":{"host":"localhost","port":1893}}'
    # sleep 1
    # mosquitto_pub -h localhost -p 1883 -t "/api/tool/distributedbroker/add" -m '{"topic":"/1","broker_info":{"host":"localhost","port":1894}}'

    # manager へ gateway の担当エリアを設定する
    sleep 1
    mosquitto_pub -h localhost -p 1883 -t "/api/tool/gatewaybroker/set" -m '{"topic":"/","broker_info":{"host":"localhost","port":1884}}'
    sleep 1
    mosquitto_pub -h localhost -p 1883 -t "/api/tool/gatewaybroker/set" -m '{"topic":"/0","broker_info":{"host":"localhost","port":1884}}'
fi

# dmb -> manager への通信確認コマンド
# mosquitto_sub -h localhost -p 1883 -t /api/tool/distributedbroker/add

# gateway -> manager への通信確認コマンド
# mosquitto_sub -h localhost -p 1883 -t /api/notice/gatewaybroker

