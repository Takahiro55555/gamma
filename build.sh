#!/bin/bash

SRCFILE="cmd/gateway/main.go"
OUTPUTFILE_DIR="bin/$(python3 -c "from datetime import datetime as dt;print(dt.now().strftime('%Y-%m-%d/%H'))")"
OUTPUTFILE="${OUTPUTFILE_DIR}/$(python3 -c "from datetime import datetime as dt;print(dt.now().strftime('%Y%m%d%H-%M-%S'))")"
OUTPUTFILE_SIG="bin/`hostname`.bin.sig"
SIG_CMD="md5sum"
if [ `uname` = "Darwin" ]; then
	SIG_CMD="md5"  # MacOS を想定
fi

if [ ! -d ${OUTPUTFILE_DIR} ]; then
    mkdir -p ${OUTPUTFILE_DIR}
fi

go build -o "${OUTPUTFILE}.out" ${SRCFILE}
GOOS=linux GOARCH=arm64 go build -o "${OUTPUTFILE}.arm64" ${SRCFILE}
GOOS=linux GOARCH=arm go build -o "${OUTPUTFILE}.arm" ${SRCFILE}

${SIG_CMD} "${OUTPUTFILE}.out" >> ${OUTPUTFILE_SIG}
${SIG_CMD} "${OUTPUTFILE}.arm64" >> ${OUTPUTFILE_SIG}
${SIG_CMD} "${OUTPUTFILE}.arm" >> ${OUTPUTFILE_SIG}

cp "${OUTPUTFILE}.out" "${OUTPUTFILE}.arm64" "${OUTPUTFILE}.arm" .
