#!/bin/bash

# configuration

OUT_DIR="api"
PROTO_DIR=$1

PACKAGE_PREFIX="github.com/aoscloud/aos_common/api"

COMMON_SOURCES="common/v1/common.proto"

IAM_SOURCES="iamanager/v5/iamanager.proto"

SM_SOURCES="servicemanager/v4/servicemanager.proto"

UM_SOURCES="updatemanager/v2/updatemanager.proto"

CM_SOURCES="communicationmanager/v3/updatescheduler.proto"

if [ "$#" -lt 1 ]; then
    echo "Usage example: $(basename -- "$0") PROTO_PATH"
    exit 1
fi

create_package_options() {
    go_opt=""

    for item in $1; do
        opt="M${item}=${2}"
        go_opt+=" --go_opt=${opt} --go-grpc_opt=${opt}"
    done

    echo "${go_opt}"
}

create_protoc_options() {
    go_opt=$(create_package_options "$1" "$2")

    go_opt+=" --proto_path=${PROTO_DIR} --go_out=${OUT_DIR} --go-grpc_out=${OUT_DIR}"

    echo "${go_opt}"
}

# clear output dir

mkdir -p ${OUT_DIR}
rm -rf ${OUT_DIR}/common ${OUT_DIR}/communicationmanager ${OUT_DIR}/iamanager ${OUT_DIR}/servicemanager ${OUT_DIR}/updatemanager

MODULE_COMMON_OPTS=$(create_package_options "${COMMON_SOURCES}" "${PACKAGE_PREFIX}/common")

# Generate common services
protoc $(create_protoc_options "${COMMON_SOURCES}" "./common") ${COMMON_SOURCES}
# Generate IAM services
protoc $(create_protoc_options "${IAM_SOURCES}" "./iamanager") ${MODULE_COMMON_OPTS} ${IAM_SOURCES}
# Generate SM services
protoc $(create_protoc_options "${SM_SOURCES}" ./servicemanager) ${MODULE_COMMON_OPTS} ${SM_SOURCES}
# Generate UM services
protoc $(create_protoc_options "${UM_SOURCES}" ./updatemanager) ${MODULE_COMMON_OPTS} ${UM_SOURCES}
# Generate CM services
protoc $(create_protoc_options "${CM_SOURCES}" ./communicationmanager) ${MODULE_COMMON_OPTS} ${CM_SOURCES}
