#!/bin/bash

# configuration

OUT_DIR="api"

IAM_SOURCES="iamanager/v4/iamanager.proto"

SM_SOURCES="servicemanager/v3/servicemanager.proto"

UM_SOURCES="updatemanager/v1/updatemanager.proto"

CM_SOURCES="communicationmanager/v2/updatescheduler.proto"

if [ "$#" -lt 1 ]; then
    echo "Usage example: $(basename -- "$0") PROTO_PATH"
    exit 1
fi

create_package_options() {
    go_opt=""

    for item in $1; do
        go_opt+=" --go_opt=M${item}=./;$2"
    done

    for item in $1; do
        go_opt+=" --go-grpc_opt=M${item}=./;$2"
    done

    echo ${go_opt}
}

COMMON_OPTIONS="--proto_path=${1} --go_out=${OUT_DIR} \
    --go_opt=paths=source_relative --go-grpc_out=${OUT_DIR} --go-grpc_opt=paths=source_relative"

# clear output dir

mkdir -p ${OUT_DIR}

rm -rf ${OUT_DIR}/communicationmanager ${OUT_DIR}/iamanager ${OUT_DIR}/servicemanager ${OUT_DIR}/updatemanager

# Generate IAM services

protoc $COMMON_OPTIONS $(create_package_options "${IAM_SOURCES}" iamanager) ${IAM_SOURCES}

# Generate SM services

protoc $COMMON_OPTIONS $(create_package_options "${SM_SOURCES}" servicemanager) ${SM_SOURCES}

# Generate UM services

protoc $COMMON_OPTIONS $(create_package_options "${UM_SOURCES}" updatemanager) ${UM_SOURCES}

# Generate CM services

protoc $COMMON_OPTIONS $(create_package_options "${CM_SOURCES}" communicationmanager) ${CM_SOURCES}
