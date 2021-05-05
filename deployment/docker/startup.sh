#!/bin/bash
set -eu

if [ "$SECRET_NODE_TYPE" == "BOOTSTRAP" ]
then
    echo 'IMMA BOOTSTRAP'
    ./bootstrap_init.sh
elif [ "$SECRET_NODE_TYPE" == "RUMOR" ]
then
    echo 'IMMA RUMOR'
    ./rumor_init.sh
else
    echo 'IMMA NODE'
    ./node_init.sh
fi
