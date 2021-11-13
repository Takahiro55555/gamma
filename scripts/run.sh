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
env=development
caller= 

if [ -f ${PID_FILE} ]; then
    cat ${PID_FILE} | xargs -I{} kill {}
    ${TEST_DOCKER_COMPOSE_DOWN}
    rm ${PID_FILE}
elif [ "${1}" = "run" ]; then
    ${TEST_DOCKER_COMPOSE_DOWN}
    ${TEST_DOCKER_COMPOSE_UP_D}
    go run cmd/manager/main.go -level ${level} \
                               -env ${env} ${caller} \
                               -host ${managerHost} \
                               -port ${managerPort} &
    echo $! > ${PID_FILE}
    go run cmd/gateway/main.go -level ${level} \
                               -env ${env} ${caller} \
                               -managerHost ${managerHost} \
                               -managerPort ${managerPort} \
                               -gatewayHost ${gatewayHost} \
                               -gatewayPort ${gatewayPort} &
    echo $! >> ${PID_FILE}
    sleep 1
    ps | grep main | grep -oE "^[0-9]+" >> ${PID_FILE}
fi
