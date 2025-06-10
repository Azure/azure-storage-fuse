#!/bin/bash

#
# This is an automated test script for testing the functional correctness of
# the distributed cache cluster under various practical node up/down
# scenarios. It runs from one of the cluster node and uses passwordless ssh
# login to other nodes to run commands on remote nodes for simulating various
# node (un) reachability scenarios.
# Here are some pre-requisites for this script:
# - passwordless ssh must be configured from any node to any node in the cluster.
# - /etc/hosts must have entries added so that vmN can be used to connect to
#   node N, f.e., vm1, vm2, etc.
#
# Q: What does this script do?
# A: It starts/stops blobfuse on various nodes and checks cluster health by
#    checking clustermap and performing filesystem operations from various
#    cluster nodes.
#

MOUNTDIR=/home/dcacheuser/mnt/
LOGDIR=/tmp/cluster_validator/

#
# some common colour escape sequences
#
RED="\e[2;31m"
RED_BOLD="\e[1;31m"
GREEN="\e[2;32m"
GREEN_BOLD="\e[1;32m"
YELLOW="\e[2;33m"
YELLOW_BOLD="\e[1;33m"
NORMAL="\e[0m"
NORMAL_BOLD="\e[0;1m"

# success echo
secho()
{
    local options

    if [ "$1" == "-n" ]; then
        options="-n"
        shift
    fi

    echo $options -e "${GREEN}${*}${NORMAL}"
}

# success bold_ echo
sbecho()
{
    local options

    if [ "$1" == "-n" ]; then
        options="-n"
        shift
    fi

    echo $options -e "${GREEN_BOLD}${*}${NORMAL}"
}

#
# warning echo
# warnings and errors go on stderr
#
wecho()
{
    local options

    if [ "$1" == "-n" ]; then
        options="-n"
        shift
    fi

    echo $options -e "${YELLOW}${*}${NORMAL}" 1>&2
}

# warning bold_ echo
wbecho()
{
    local options

    if [ "$1" == "-n" ]; then
        options="-n"
        shift
    fi

    echo $options -e "${YELLOW_BOLD}${*}${NORMAL}" 1>&2
}

# error echo
eecho()
{
    local options

    if [ "$1" == "-n" ]; then
        options="-n"
        shift
    fi

    echo $options -e "${RED}${*}${NORMAL}" 1>&2
}

# error bold_ echo
ebecho()
{
    local options

    if [ "$1" == "-n" ]; then
        options="-n"
        shift
    fi

    echo $options -e "${RED_BOLD}${*}${NORMAL}" 1>&2
}

# bold_ echo
becho()
{
    local options

    if [ "$1" == "-n" ]; then
        options="-n"
        shift
    fi

    echo $options -e "${NORMAL_BOLD}${*}${NORMAL}"
}

# Call it with the status of the last executed command to print a coloured ok/failed status on the same line.
log_status()
{
    local status=$1
    local err_msg=$2

    if [ $status -eq 0 ]; then
        sbecho  "\033[50D\033[70C[ok]"
    else
        ebecho "\033[50D\033[70C[failed]"

        # Log error message if provided.
        if [ -n "$err_msg" ]; then
            ebecho "$err_msg"
        fi

        exit 1
    fi
}

vmlog()
{
    local vm=$1
    echo $LOGDIR/$vm.log
}

get_node_id()
{
    local vm=$1
    local logfile=$(vmlog $vm)

    ssh $vm cat ~/.blobfuse2/blobfuse_node_uuid

    if [ $? -ne 0 ]; then
        echo "Cannot read nodeid"
        exit 1
    fi
}

wait_till_next_epoch()
{
    local next_epoch=$(expr $LAST_UPDATED_AT + $CLUSTERMAP_EPOCH)
    local now=$(date +%s)
    local secs_to_next_epoch=$(expr $next_epoch - $now + 2)

    if [ $secs_to_next_epoch -le 0 ]; then
        wbecho "Next epoch already over"
        return
    fi

    echo "Sleeping $secs_to_next_epoch seconds till next epoch..."
    sleep $secs_to_next_epoch
    echo "Done"
}

start_blobfuse_on_node()
{
    local vm=$1
    local logfile=$(vmlog $vm)

    (
        echo "Starting blobfuse @ $(date)" >> $logfile
        ssh $vm ~/start-blobfuse.sh >> $logfile 2>&1
    )&

    # Give some time for blobfuse process to start.
    sleep 1
}

stop_blobfuse_on_node()
{
    local vm=$1
    local logfile=$(vmlog $vm)

    (
        echo "Stopping blobfuse @ $(date)" >> $logfile
        ssh $vm ~/stop-blobfuse.sh >> $logfile 2>&1
}

kill_blobfuse_on_node()
{
    local vm=$1
    local logfile=$(vmlog $vm)

    echo "Killing blobfuse @ $(date)" >> $logfile
    ssh $vm pkill blobfuse2 >> $logfile 2>&1
}

#
# Simulate node up by unblocking RPC port 9090 and starting blobfuse2.
#
node_up()
{
    local vm=$1
    local logfile=$(vmlog $vm)

    echo "Starting $vm @ $(date)" >> $logfile
    ssh $vm ~/block-rpc.sh unblock >> $logfile 2>&1

    start_blobfuse_on_node $vm
}

#
# Simulate node down by killing blobfuse2 and blocking RPC port 9090
#
node_down()
{
    local vm=$1
    local logfile=$(vmlog $vm)

    kill_blobfuse_on_node $vm

    echo "Stopping $vm @ $(date)" >> $logfile
    ssh $vm ~/block-rpc.sh block >> $logfile 2>&1
}

read_clustermap_from_node()
{
    local vm=$1
    local logfile=$(vmlog $vm)

    clustermap_path="$MOUNTDIR/fs=debug/clustermap"

    echo "[cat $clustermap_path] @ $(date)" >> $logfile

    # Return status of cat can be checked by caller.
    ssh $vm "cat $clustermap_path" 2>>$logfile | tee -a $logfile
}

#
# Given a clustermap, return the state of the given RV.
#
get_rv_state()
{
    local cm="$1"
    local rv=$2

    echo "$cm" | jq '."rv-list"[] | to_entries[] | select(.key | startswith("'$rv'")).value.state' | tr -d '"'
}

#
# Given a clustermap, return the count of RVs in rv-list.
#
get_rv_count()
{
    local cm="$1"

    echo "$cm" | jq '."rv-list" | length'
}

#
# Given a clustermap, return the count of MVs in mv-list.
#
get_mv_count()
{
    local cm="$1"

    echo "$cm" | jq '."mv-list" | length'
}

#
# Action starts here
#
mkdir -p $LOGDIR

############################################################################
##                             Start node1                                ##
############################################################################

echo
wbecho ">> Starting blobfuse on vm1"
echo
start_blobfuse_on_node vm1

#
# As soon as we start blobfuse on the first node, it should update the clustermap with its rv
#
becho -n "Reading clustermap on vm1"
cm=$(read_clustermap_from_node vm1)
log_status $?

# Save some config variables, for later use.
CLUSTERMAP_EPOCH=$(echo "$cm" | jq '."config"."clustermap-epoch"')
MIN_NODES=$(echo "$cm" | jq '."config"."min-nodes"')
NUM_REPLICAS=$(echo "$cm" | jq '."config"."num-replicas"')
HB_SECONDS=$(echo "$cm" | jq '."config"."heartbeat-seconds"')
HB_TILL_NODE_DOWN=$(echo "$cm" | jq '."config"."heartbeats-till-node-down"')
LAST_UPDATED_AT=$(echo "$cm" | jq '."config"."last_updated_at"')

echo
echo -e "clustermap-epoch:\033[50D\033[30C$CLUSTERMAP_EPOCH"
echo -e "min-nodes:\033[50D\033[30C$MIN_NODES"
echo -e "num-replicas:\033[50D\033[30C$NUM_REPLICAS"
echo -e "heartbeat-seconds:\033[50D\033[30C$HB_SECONDS"
echo -e "heartbeats-till-node-down:\033[50D\033[30C$HB_TILL_NODE_DOWN"
echo

becho -n "last_updated_by must be vm1"
LAST_UPDATED_BY=$(echo "$cm" | jq '."last_updated_by"' | tr -d '"')
[ "$LAST_UPDATED_BY" == "$(get_node_id vm1)" ]
log_status $? "is $LAST_UPDATED_BY"

becho -n "last_updated_at must be uptodate"
LAST_UPDATED_AT=$(echo "$cm" | jq '."last_updated_at"')
now=$(date +%s)
# Not more than 5secs old.
[ $(expr $now - $LAST_UPDATED_AT) -lt 5 ]
log_status $? "now is $now and last_updated_at is $LAST_UPDATED_AT"

becho -n "Cluster state must be ready"
cluster_state=$(echo "$cm" | jq '."state"' | tr -d '"')
[ "$cluster_state" == "ready" ]
log_status $? "is $cluster_state"

# Epoch is 1 for initial clustermap and then updated by 1 when RV is added to rv-list.
becho -n "Epoch must be 2"
LAST_EPOCH=$(echo "$cm" | jq '."epoch"')
[ "$LAST_EPOCH" == "2" ]
log_status $? "is $LAST_EPOCH"

becho -n "rv0 must be online"
rv0_state=$(get_rv_state "$cm" "rv0")
[ "$rv0_state" == "online" ]
log_status $? "is $rv0_state"

becho -n "RV count must be 1"
rv_count=$(get_rv_count "$cm")
[ "$rv_count" == "1" ]
log_status $? "is $rv_count"

becho -n "Cluster must be readonly"
readonly_status=$(echo "$cm" | jq '."readonly"')
if [ "$rv_count" -ge "$MIN_NODES" ]; then
    [ "$readonly_status" == "false" ]
else
    [ "$readonly_status" == "true" ]
fi
log_status $? "rv_count is $rv_count, min_nodes is $MIN_NODES, readonly is $readonly_status"

############################################################################
##                             Start node2                                ##
############################################################################

echo
wbecho ">> Starting blobfuse on vm2"
echo
start_blobfuse_on_node vm2

#
# As soon as we start blobfuse on node2, it should update the clustermap with its rv, but
# node1 will come to know about the updated clustermap only when it refreshes the clustermap
# on next epoch.
#
becho -n "Reading clustermap on vm1"
cm=$(read_clustermap_from_node vm1)
log_status $?

becho -n "RV count must be 1"
rv_count=$(get_rv_count "$cm")
[ "$rv_count" == "1" ]
log_status $? "is $rv_count"

becho -n "Reading clustermap on vm2"
cm=$(read_clustermap_from_node vm2)
log_status $?

becho -n "last_updated_by must be vm2"
last_updated_by=$(echo "$cm" | jq '."last_updated_by"' | tr -d '"')
[ "$last_updated_by" == "$(get_node_id vm2)" ]
log_status $? "is $last_updated_by"

becho -n "last_updated_at must be uptodate"
last_updated_at=$(echo "$cm" | jq '."last_updated_at"')
now=$(date +%s)
# Not more than 5secs old.
[ $(expr $now - $last_updated_at) -lt 5 ]
log_status $? "now is $now and last_updated_at is $last_updated_at"

becho -n "Cluster state must be ready"
cluster_state=$(echo "$cm" | jq '."state"' | tr -d '"')
[ "$cluster_state" == "ready" ]
log_status $? "is $cluster_state"

# vm2 nust have updated it once.
becho -n "Epoch must be 3"
epoch=$(echo "$cm" | jq '."epoch"')
[ "$epoch" == "3" ]
log_status $? "is $epoch"

becho -n "rv0 must be online"
rv0_state=$(get_rv_state "$cm" "rv0")
[ "$rv0_state" == "online" ]
log_status $? "is $rv0_state"

becho -n "rv1 must be online"
rv1_state=$(get_rv_state "$cm" "rv1")
[ "$rv1_state" == "online" ]
log_status $? "is $rv1_state"

becho -n "RV count must be 2"
rv_count=$(get_rv_count "$cm")
[ "$rv_count" == "2" ]
log_status $? "is $rv_count"

becho -n "Cluster must be readonly"
readonly_status=$(echo "$cm" | jq '."readonly"')
if [ "$rv_count" -ge "$MIN_NODES" ]; then
    [ "$readonly_status" == "false" ]
else
    [ "$readonly_status" == "true" ]
fi
log_status $? "rv_count is $rv_count, min_nodes is $MIN_NODES, readonly is $readonly_status"

#
# Wait for clustermap update on vm1.
# After that it'll get the clustermap updated by vm2.
#
wait_till_next_epoch

becho -n "Reading clustermap on vm1"
cm=$(read_clustermap_from_node vm1)
log_status $?

becho -n "RV count must be 2"
rv_count=$(get_rv_count "$cm")
[ "$rv_count" == "2" ]
log_status $? "is $rv_count"

LAST_UPDATED_AT=$(echo "$cm" | jq '."last_updated_at"')
LAST_EPOCH=$(echo "$cm" | jq '."epoch"')

############################################################################
##                             Start node3                                ##
############################################################################

echo
wbecho ">> Starting blobfuse on vm3"
echo
start_blobfuse_on_node vm3

#
# As soon as we start blobfuse on node3, it should update the clustermap with its rv, but
# node1 and node2 will come to know about the updated clustermap only when they refreshe
# their clustermaps on their next epochs.
#
becho -n "Reading clustermap on vm1"
cm=$(read_clustermap_from_node vm1)
log_status $?

becho -n "RV count must be 2"
rv_count=$(get_rv_count "$cm")
[ "$rv_count" == "2" ]
log_status $? "is $rv_count"

becho -n "Reading clustermap on vm2"
cm=$(read_clustermap_from_node vm2)
log_status $?

becho -n "RV count must be 2"
rv_count=$(get_rv_count "$cm")
[ "$rv_count" == "2" ]
log_status $? "is $rv_count"


becho -n "Reading clustermap on vm3"
cm=$(read_clustermap_from_node vm3)
log_status $?

becho -n "last_updated_by must be vm3"
last_updated_by=$(echo "$cm" | jq '."last_updated_by"' | tr -d '"')
[ "$last_updated_by" == "$(get_node_id vm3)" ]
log_status $? "is $last_updated_by"

becho -n "last_updated_at must be uptodate"
last_updated_at=$(echo "$cm" | jq '."last_updated_at"')
now=$(date +%s)
# Not more than 5secs old.
[ $(expr $now - $last_updated_at) -lt 5 ]
log_status $? "now is $now and last_updated_at is $last_updated_at"

#
# Cluster will still be readonly, it'll be marked read-write when the next leader node
# updates the clustermap including creating new MVs
#
becho -n "Cluster must be readonly"
readonly_status=$(echo "$cm" | jq '."readonly"')
if [ "$rv_count" -ge "$MIN_NODES" ]; then
    [ "$readonly_status" == "false" ]
else
    [ "$readonly_status" == "true" ]
fi
log_status $? "rv_count is $rv_count, min_nodes is $MIN_NODES, readonly is $readonly_status"

becho -n "Cluster state must be ready"
cluster_state=$(echo "$cm" | jq '."state"' | tr -d '"')
[ "$cluster_state" == "ready" ]
log_status $? "is $cluster_state"

# vm3 nust have updated it once.
becho -n "Epoch must be 4"
epoch=$(echo "$cm" | jq '."epoch"')
[ "$epoch" == "4" ]
log_status $? "is $epoch"

becho -n "rv0 must be online"
rv0_state=$(get_rv_state "$cm" "rv0")
[ "$rv0_state" == "online" ]
log_status $? "is $rv0_state"

becho -n "rv1 must be online"
rv1_state=$(get_rv_state "$cm" "rv1")
[ "$rv1_state" == "online" ]
log_status $? "is $rv1_state"

becho -n "rv2 must be online"
rv2_state=$(get_rv_state "$cm" "rv2")
[ "$rv2_state" == "online" ]
log_status $? "is $rv2_state"

becho -n "RV count must be 3"
rv_count=$(get_rv_count "$cm")
[ "$rv_count" == "2" ]
log_status $? "is $rv_count"

becho -n "Cluster must be read-write ready"
readonly_status=$(echo "$cm" | jq '."readonly"')
if [ "$rv_count" -ge "$MIN_NODES" ]; then
    [ "$readonly_status" == "false" ]
else
    [ "$readonly_status" == "true" ]
fi
log_status $? "rv_count is $rv_count, min_nodes is $MIN_NODES, readonly is $readonly_status"

#
# Wait for clustermap update on vm1.
# After that it'll get the clustermap updated by vm3.
#
wait_till_next_epoch

becho -n "Reading clustermap on vm1"
cm=$(read_clustermap_from_node vm1)
log_status $?

becho -n "RV count must be 3"
rv_count=$(get_rv_count "$cm")
[ "$rv_count" == "3" ]
log_status $? "is $rv_count"

LAST_UPDATED_AT=$(echo "$cm" | jq '."last_updated_at"')
LAST_EPOCH=$(echo "$cm" | jq '."epoch"')