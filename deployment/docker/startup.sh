#!/bin/bash
set -eu

if [ "$SECRET_NODE_TYPE" == "BOOTSTRAP" ]
then
    ./bootstrap_init.sh
elif [ "$SECRET_NODE_TYPE" == "RUMOR" ]
then
    ./rumor_init.sh
else
    ./node_init.sh
fi
