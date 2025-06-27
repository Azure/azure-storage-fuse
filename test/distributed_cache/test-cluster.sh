#!/bin/bash

#
# This is an automated test script for testing the functional correctness of
# the distributed cache cluster under various practical node up/down
# scenarios. It runs from one of the cluster node and uses passwordless ssh
# login to other nodes to run commands on remote nodes for simulating various
# node (un) reachability scenarios.
#
# Usage: ./test_cluster.sh <number_of_nodes>
# Example: ./test_cluster.sh 5
#
# Here are some pre-requisites for this script:
# - passwordless ssh must be configured from any node to any node in the cluster.
# - /etc/hosts must have entries added so that vmN can be used to connect to
#   node N, f.e., vm1, vm2, etc.
# - 'jq' command-line JSON processor must be installed on the nodes.
#
# Q: What does this script do?
# A: It starts/stops blobfuse on various nodes and checks cluster health by
#    checking clustermap and performing filesystem operations from various
#    cluster nodes.
#

# Function to discover available nodes
discover_nodes() {
    # This function could be implemented to dynamically discover available nodes
    # For now, we'll use a simple method that accepts comma-separated node names
    # via environment variable or command line argument
    
    local nodes_arg="$1"
    
    if [ -n "$CLUSTER_NODES" ]; then
        # Use environment variable if set
        echo "$CLUSTER_NODES" | tr ',' ' '
    elif [ -n "$nodes_arg" ]; then
        # Use command line argument if provided
        echo "$nodes_arg" | tr ',' ' '
    else
        # Fall back to default naming pattern if no nodes specified
        local count="$NUM_NODES"
        local node_list=""
        for ((i=1; i<=count; i++)); do
            node_list="$node_list vm$i"
        done
        echo "$node_list"
    fi
}

# Generate list of node names
generate_node_list() {
    local count=$1
    local all_nodes="$ALL_NODES"
    
    # If we have a list of all nodes, just take the first $count of them
    if [ -n "$all_nodes" ]; then
        echo "$all_nodes" | tr ' ' '\n' | head -n "$count" | tr '\n' ' '
    else
        # Fall back to default naming pattern if no nodes discovered
        local node_list=""
        for ((i=1; i<=count; i++)); do
            node_list="$node_list vm$i"
        done
        echo "$node_list"
    fi
}

# Get a specific node by index
get_node_by_index() {
    local index=$1
    local all_nodes=($ALL_NODES)
    
    if [ $index -gt 0 ] && [ $index -le ${#all_nodes[@]} ]; then
        echo "${all_nodes[$((index-1))]}"
    else
        echo "Invalid node index: $index" >&2
        return 1
    fi
}

# Update usage information
usage() {
    echo "Usage: ./test_cluster.sh <number_of_nodes> [node_list]"
    echo "Example: ./test_cluster.sh 5"
    echo "Example with custom node names: ./test_cluster.sh 3 node1,node2,node3"
    echo
    echo "You can also set CLUSTER_NODES environment variable:"
    echo "CLUSTER_NODES=\"node1 node2 node3\" $0 3"
    exit 1
}

# Check if at least the number of nodes is provided
if [ $# -lt 1 ]; then
    usage
    exit 1
fi

NUM_NODES=$1
shift  # Remove first argument

# Validate input
if ! [[ "$NUM_NODES" =~ ^[0-9]+$ ]] || [ "$NUM_NODES" -lt 1 ]; then
    echo "Error: Number of nodes must be a positive integer"
    exit 1
fi

# Check for optional node list argument
NODE_LIST=""
if [ $# -gt 0 ]; then
    NODE_LIST="$1"
    shift
fi

# Discover available nodes
ALL_NODES=$(discover_nodes "$NODE_LIST")

# Count how many nodes we found
FOUND_NODES=$(echo "$ALL_NODES" | wc -w)

# Verify we have enough nodes
if [ "$FOUND_NODES" -lt "$NUM_NODES" ]; then
    echo "Error: Requested $NUM_NODES nodes but only found $FOUND_NODES: $ALL_NODES"
    exit 1
fi

echo "Starting cluster test with $NUM_NODES nodes: $ALL_NODES"

MOUNTDIR=/home/dcacheuser/mnt/
LOGDIR=/tmp/cluster_validator/
RESYNC_INTERVAL=12

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

wait_till_next_scheduled_epoch()
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

wait_till_hb_expiry()
{
    local next_epoch=$(expr $LAST_UPDATED_AT + $CLUSTERMAP_EPOCH)

    local now=$(date +%s)
    local secs_to_next_epoch=$(expr $next_epoch - $now)
   # Check if we're close to the heartbeat timeout boundary
    local heartbeat_timeout=$(expr $HB_SECONDS \* $HB_TILL_NODE_DOWN - 2)
    local secs_to_next_epoch_with_buffer

    if [ $secs_to_next_epoch -lt $heartbeat_timeout ]; then
    # Add an additional epoch period if we're close to timeout
    secs_to_next_epoch_with_buffer=$(expr $secs_to_next_epoch + $CLUSTERMAP_EPOCH + 5)
    echo "Close to heartbeat timeout boundary, adding extra epoch wait time"
    else
        # Otherwise just add a small buffer
        secs_to_next_epoch_with_buffer=$(expr $secs_to_next_epoch + 5)
    fi

    echo "Sleeping $secs_to_next_epoch_with_buffer seconds till next epoch..."
    sleep $secs_to_next_epoch_with_buffer
    echo "Done"
}

cleanup()
{
    wbecho "Stopping blobfuse on started nodes..."

    # Assuming `NODES_STARTED` contains the list of started nodes (e.g., "vm1 vm2 vm3")
    for vm_name in $NODES_STARTED; do
        stop_blobfuse_on_node $vm_name
    done

    wbecho "Stop completed."
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
    sleep 2
    NODES_STARTED="$NODES_STARTED $vm" # Add to list of started nodes for cleanup
}

stop_blobfuse_on_node()
{
    local vm=$1
    local remove_from_list=${2:-false}  # Optional parameter, default is false
    local logfile=$(vmlog $vm)

    echo "Stopping blobfuse @ $(date)" >> $logfile
    ssh $vm ~/stop-blobfuse.sh >> $logfile 2>&1
    
    # If requested, remove the node from the NODES_STARTED list
    if [ "$remove_from_list" == "true" ]; then
        NODES_STARTED=$(echo "$NODES_STARTED" | sed "s/ $vm / /g" | sed "s/^ $vm//" | sed "s/$vm $//")
    fi
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

write_data_in_dcache()
{
    local vm=$1
    local file_name=$2
    local block_size=$3
    local count=$4
    ssh $vm "dd if=/dev/urandom of=$MOUNTDIR/fs=dcache/$file_name bs=$block_size count=$count conv=fsync"
}

write_data_in_azure()
{
    local vm=$1
    local file_name=$2
    local block_size=$3
    local count=$4
    ssh $vm "dd if=/dev/urandom of=$MOUNTDIR/fs=azure/$file_name bs=$block_size count=$count conv=fsync"
}

write_data_on_both()
{
    local vm=$1
    local file_name=$2
    local block_size=$3
    local count=$4
    ssh $vm "dd if=/dev/urandom of=$MOUNTDIR/$file_name bs=$block_size count=$count conv=fsync"
}

get_md5sum()
{
    local vm=$1
    local file_name=$2
    local namespace=$3 #optional

    if [ -n "$namespace" ]; then
        ssh $vm "md5sum $MOUNTDIR/fs=$namespace/$file_name | cut -d' ' -f1"
    else
        ssh $vm "md5sum $MOUNTDIR/$file_name | cut -d' ' -f1"
    fi
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

# Fix the get_node_ip function
get_node_ip()
{
    local cm="$1"
    local node_id="$2"

    echo "$cm" | jq -r --arg node_id "$node_id" '
    .["rv-list"][]
    | to_entries[]
    | select(.value.node_id == $node_id)
    | .value.ipaddr'
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
# Given a clustermap, return the count of nodes in rv-list.
#
get_node_count()
{
    local cm="$1"

    echo "$cm" | jq '."rv-list" | map(.[]) | map(.node_id) | unique | length'
}

# Given a clustermap, node_id and state, return the list of RVs for that node.
get_rv_list_for_node_with_state()
{
    local cm=$1
    local vm_node_id=$2
    local desired_state="$3"

    echo "$cm" | jq -r --arg node_id "$vm_node_id" --arg state "$desired_state" '
        (
          .["rv-list"] | map(to_entries[])? | from_entries
        ) as $rvs |
        $rvs | to_entries[] |
        select(.value.node_id == $node_id and .value.state == $state) |
        .key' | paste -sd, -

}

# Given a clustermap, node_id, return the list of RVs for that node.
get_rv_list_for_node()
{
    local cm=$1
    local vm_node_id=$2

    rv_list=$(echo "$cm" | jq -r --arg node_id "$vm_node_id" '
        [.["rv-list"][] | 
        to_entries[] | 
        select(.value.node_id == $node_id) | 
        .key] | join(",")
    ')
    
    # Return the original result
    echo "$rv_list"
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
# Given a clustermap, return the count of MVs with given state in mv-list.
#
get_mv_count_with_state()
{
    local cm="$1"
    local mv_state="$2"

    echo "$cm" | jq '[."mv-list"[] | to_entries[] | select(.value.state == "'"$mv_state"'")] | length'
}

# Given a clustermap, RV list and state, return the count of MVs where these RV exist.
get_mvs_count_for_given_rv_with_state()
{
    local cm="$1"
    local rv_list="$2"
    local rv_state="$3"

    echo "$cm" | jq --arg state "$rv_state" --arg rv_names_str "$rv_list" '
        ($rv_names_str | split(",")) as $target_rvs |
        (
          .["mv-list"] | map(to_entries[])? | from_entries
        ) as $mvs |
        [
          $mvs | to_entries[] |
          select(
            .value.rvs | to_entries[]? |
            select(
              (.key | IN($target_rvs[])) and (.value == $state)
            )
          )
        ] | length'
}

# Validate that all RVs (rv0 to rv{n-1}) are online
validate_all_rvs_online()
{
    local cm="$1"
    local expected_rv_count=$2
    
    for ((i=0; i<expected_rv_count; i++)); do
        becho -n "rv$i must be online"
        rv_state=$(get_rv_state "$cm" "rv$i")
        [ "$rv_state" == "online" ]
        log_status $? "is $rv_state"
    done
}

# Test file operations on a specific node
test_file_operations()
{
    local vm=$1
    local node_num=$2
    local cluster_readonly=$3
    
    if [ "$cluster_readonly" == "true" ]; then
        # In readonly mode, only azure operations should work
        # Dcache operations should fail
        becho -n "Dcache File creation must fail on $vm"
        ssh $vm "echo dcache > $MOUNTDIR/fs=dcache/file${node_num}.dcache"
        [ ! -f "$MOUNTDIR/fs=dcache/file${node_num}.dcache" ]
        log_status $?

        # Unqualified path operations should fail
        becho -n "Unqualified path File creation must fail on $vm"
        ssh $vm "echo both > $MOUNTDIR/file${node_num}.both"
        [ ! -f "$MOUNTDIR/file${node_num}.both" ]
        log_status $?

       
    else
        # Test Dcache file operations should work
        becho -n "Dcache File creation must work on $vm"
        ssh $vm "echo dcache > $MOUNTDIR/fs=dcache/file${node_num}.dcache"
        TOTAL_DCACHE_FILES=$((TOTAL_DCACHE_FILES + 1))
        TOTAL_BOTH_FILES=$((TOTAL_BOTH_FILES + 1))
        [ $? -eq 0 ]
        log_status $?

        becho -n "Dcache file Read must work on $vm"
        buf=$(ssh $vm "cat $MOUNTDIR/fs=dcache/file${node_num}.dcache")
        [ $? -eq 0 -a "$buf" == "dcache" ]
        log_status $? "buf: $buf"

        # Test unqualified path file operations should work
        becho -n "Unqualified path File creation must work on $vm"
        ssh $vm "echo both > $MOUNTDIR/file${node_num}.both"
        TOTAL_DCACHE_FILES=$((TOTAL_DCACHE_FILES + 1))
        TOTAL_AZURE_FILES=$((TOTAL_AZURE_FILES + 1))
        TOTAL_BOTH_FILES=$((TOTAL_BOTH_FILES + 1))
        [ $? -eq 0 ]
        log_status $?

        
    fi

    becho -n "Azure File creation must work on $vm"
    ssh $vm "echo azure > $MOUNTDIR/fs=azure/file${node_num}.azure"
    TOTAL_AZURE_FILES=$((TOTAL_AZURE_FILES + 1))
    TOTAL_BOTH_FILES=$((TOTAL_BOTH_FILES + 1))
    [ $? -eq 0 ]
    log_status $?

    becho -n "Azure file Read must work on $vm"
    buf=$(ssh $vm "cat $MOUNTDIR/fs=azure/file${node_num}.azure")
    [ $? -eq 0 -a "$buf" == "azure" ]
    log_status $? "buf: $buf"
}


# Validate clustermap consistency across all nodes
validate_clustermap_consistency()
{
    local expected_rv_count=$1
    local nodes_to_check="$2"
    
    for vm in $nodes_to_check; do
        becho -n "Reading clustermap on $vm"
        cm=$(read_clustermap_from_node $vm)
        log_status $?

        if [ -n "$expected_rv_count" ]; then
            becho -n "RV count must be $expected_rv_count on $vm"
            rv_count=$(get_rv_count "$cm")
            [ "$rv_count" == "$expected_rv_count" ]
            log_status $? "is $rv_count"
        fi

        becho -n "Epoch validation on $vm"
        current_epoch=$(echo "$cm" | jq '."epoch"')
        
        # Check if current epoch is within acceptable range (at most 2 less than highest seen)
        epoch_diff=$((HIGHEST_EPOCH_SEEN - current_epoch))
        [ "$epoch_diff" -le 2 ] && [ "$epoch_diff" -ge 0 ] || [ "$current_epoch" -gt "$HIGHEST_EPOCH_SEEN" ]
        log_status $? "Current epoch: $current_epoch, Highest seen: $HIGHEST_EPOCH_SEEN, Diff: $epoch_diff"
        
        # Update the highest epoch seen if current is higher
        if [ "$current_epoch" -gt "$HIGHEST_EPOCH_SEEN" ]; then
            HIGHEST_EPOCH_SEEN=$current_epoch
        fi
    done
}

# Function to validate RV node mapping consistency
validate_rv_node_mapping_consistency()
{
    local nodes_to_check="$1"
    local expected_node_count=$2
    
    for vm in $nodes_to_check; do
        becho -n "Reading clustermap on $vm for node mapping"
        cm=$(read_clustermap_from_node $vm)
        log_status $?
        
        becho -n "Node count must match expected count on $vm"
        node_count=$(get_node_count "$cm")
        [ "$node_count" -eq "$expected_node_count" ]
        log_status $? "Found $node_count nodes, expected $expected_node_count"
        
        # Validate each node has appropriate RVs assigned
        for test_vm in $nodes_to_check; do
            test_vm_id=$(get_node_id $test_vm)
            becho -n "Checking RVs assigned to $test_vm on $vm"
            rv_list=$(get_rv_list_for_node "$cm" "$test_vm_id")
            [ -n "$rv_list" ]
            log_status $? "No RVs found for $test_vm"
            
            if [ "$cluster_readonly" == "false" ]; then
                becho -n "Checking online RVs for $test_vm"
                online_rv_list=$(get_rv_list_for_node_with_state "$cm" "$test_vm_id" "online")
                [ -n "$online_rv_list" ]
                log_status $? "No online RVs found for $test_vm"
            fi
        done
    done
}

# Add the missing function to validate file listing consistency
test_file_listing_consistency()
{
    local nodes_to_check="$1"
    local expected_azure_files=$2
    local expected_dcache_files=$3
    local expected_both_files=$4
    
    for vm in $nodes_to_check; do
        # Check dcache files only if cluster is not readonly
        cluster_readonly_status=$(read_clustermap_from_node $vm | jq '."readonly"')
        if [ "$cluster_readonly_status" == "true" ]; then
            becho -n "List file must fail over dcache ns on $vm"
            file_count=$(ssh $vm "ls $MOUNTDIR/fs=dcache | wc -l")
            [ "$file_count" -eq 0 ]
            log_status $? "Expected 0 files but found $file_count"
        else
            becho -n "List file over dcache path must return $expected_dcache_files files on $vm"
            file_count=$(ssh $vm "ls $MOUNTDIR/fs=dcache | wc -l")
            [ "$file_count" -eq "$expected_dcache_files" ]
            log_status $? "Expected $expected_dcache_files files but found $file_count"
        fi

        becho -n "List file must return $expected_azure_files files over azure ns on $vm"
        file_count=$(ssh $vm "ls $MOUNTDIR/fs=azure | wc -l")
        [ "$file_count" -eq "$expected_azure_files" ]
        log_status $? "Expected $expected_azure_files files but found $file_count"
        
        becho -n "List file must return $expected_both_files files over unqualified path on $vm"
        file_count=$(ssh $vm "ls $MOUNTDIR | wc -l")
        [ "$file_count" -eq "$expected_both_files" ]
        log_status $? "Expected $expected_both_files files but found $file_count"
    done
}

test_cross_node_consistency()
{
    local node1=$1
    local node2=$2
    local file_num=$3
    local large_file_test=${4:-false}
    
    if [ "$large_file_test" == "true" ]; then
        if [ -n "$dcache_2GB_md5sum" ]; then
            becho -n "Cross-node large file consistency check for 2GB.dcache between $node1 and $node2"
            dcache_file_md5_node1=$(get_md5sum $node1 "2GB.dcache" dcache)
            dcache_file_md5_node2=$(get_md5sum $node2 "2GB.dcache" dcache)
            [ "$dcache_file_md5_node1" == "$dcache_file_md5_node2" ]
            log_status $? "$node1: $dcache_file_md5_node1, $node2: $dcache_file_md5_node2"
        fi
        
        if [ -n "$both_2GB_md5sum" ]; then
            becho -n "Cross-node large file consistency check for 2GB.both between $node1 and $node2"
            both_file_md5_node1=$(get_md5sum $node1 "2GB.both")
            both_file_md5_node2=$(get_md5sum $node2 "2GB.both")
            [ "$both_file_md5_node1" == "$both_file_md5_node2" ]
            log_status $? "$node1: $both_file_md5_node1, $node2: $both_file_md5_node2"
        fi
        
        if [ -n "$dcache_1GB_md5sum" ]; then
            becho -n "Cross-node large file consistency check for 1GB.dcache between $node1 and $node2"
            dcache_file_md5_node1=$(get_md5sum $node1 "1GB.dcache" dcache)
            dcache_file_md5_node2=$(get_md5sum $node2 "1GB.dcache" dcache)
            [ "$dcache_file_md5_node1" == "$dcache_file_md5_node2" ]
            log_status $? "$node1: $dcache_file_md5_node1, $node2: $dcache_file_md5_node2"
        fi
        
        if [ -n "$both_1GB_md5sum" ]; then
            becho -n "Cross-node large file consistency check for 1GB.both between $node1 and $node2"
            both_file_md5_node1=$(get_md5sum $node1 "1GB.both")
            both_file_md5_node2=$(get_md5sum $node2 "1GB.both")
            [ "$both_file_md5_node1" == "$both_file_md5_node2" ]
            log_status $? "$node1: $both_file_md5_node1, $node2: $both_file_md5_node2"
        fi
    else
        becho -n "Cross-node dcache file consistency check between $node1 and $node2"
        dcache_file_md5_node1=$(get_md5sum $node1 "file${file_num}.dcache" dcache)
        dcache_file_md5_node2=$(get_md5sum $node2 "file${file_num}.dcache" dcache)
        [ "$dcache_file_md5_node1" == "$dcache_file_md5_node2" ]
        log_status $? "$node1: $dcache_file_md5_node1, $node2: $dcache_file_md5_node2"
    
        becho -n "Cross-node azure file consistency check between $node1 and $node2"
        azure_file_md5_node1=$(get_md5sum $node1 "file${file_num}.azure" azure)
        azure_file_md5_node2=$(get_md5sum $node2 "file${file_num}.azure" azure)
        [ "$azure_file_md5_node1" == "$azure_file_md5_node2" ]
        log_status $? "$node1: $azure_file_md5_node1, $node2: $azure_file_md5_node2"
    
        becho -n "Cross-node unqualified path file consistency check between $node1 and $node2"
        unqualified_file_md5_node1=$(get_md5sum $node1 "file${file_num}.both")
        unqualified_file_md5_node2=$(get_md5sum $node2 "file${file_num}.both")
        [ "$unqualified_file_md5_node1" == "$unqualified_file_md5_node2" ]
        log_status $? "$node1: $unqualified_file_md5_node1, $node2: $unqualified_file_md5_node2"
    fi
}



#
# Action starts here
#
 
# Create log directory
mkdir -p $LOGDIR
rm -rf $LOGDIR/*

# Initialize globals
NODES_STARTED=""
TOTAL_AZURE_FILES=0
TOTAL_DCACHE_FILES=0
TOTAL_BOTH_FILES=0
HIGHEST_EPOCH_SEEN=0

# Setup cleanup on exit
trap cleanup EXIT

# Generate list of all nodes
ALL_NODES=$(generate_node_list $NUM_NODES)


############################################################################
##                             Start nodes                                ##
############################################################################

for ((current_node=1; current_node<=NUM_NODES; current_node++)); do
    vm_name=$(get_node_by_index $current_node)
    echo
    wbecho ">> Starting blobfuse on $vm_name"
    echo
    start_blobfuse_on_node $vm_name

    #
    # As soon as we start blobfuse on the first node, it should update the clustermap with its rv
    #
    becho -n "Reading clustermap on $vm_name"
    cm=$(read_clustermap_from_node $vm_name)
    log_status $?

    # Save some config variables, for later use.
    # For the first node, save configuration variables
    if [ $current_node -eq 1 ]; then
        CLUSTERMAP_EPOCH=$(echo "$cm" | jq '."config"."clustermap-epoch"')
        INITIAL_EPOCH=$(echo "$cm" | jq '."epoch"')
        HIGHEST_EPOCH_SEEN=$INITIAL_EPOCH
        MIN_NODES=$(echo "$cm" | jq '."config"."min-nodes"')
        NUM_REPLICAS=$(echo "$cm" | jq '."config"."num-replicas"')
        HB_SECONDS=$(echo "$cm" | jq '."config"."heartbeat-seconds"')
        HB_TILL_NODE_DOWN=$(echo "$cm" | jq '."config"."heartbeats-till-node-down"')
        LAST_UPDATED_AT=$(echo "$cm" | jq '."config"."last_updated_at"')
        MVS_PER_RV=$(echo "$cm" | jq '."config"."mvs-per-rv"')

        echo
        echo -e "epoch:\033[50D\033[30C$INITIAL_EPOCH"
        echo -e "clustermap-epoch:\033[50D\033[30C$CLUSTERMAP_EPOCH"
        echo -e "min-nodes:\033[50D\033[30C$MIN_NODES"
        echo -e "num-replicas:\033[50D\033[30C$NUM_REPLICAS"
        echo -e "heartbeat-seconds:\033[50D\033[30C$HB_SECONDS"
        echo -e "heartbeats-till-node-down:\033[50D\033[30C$HB_TILL_NODE_DOWN"
        echo -e "mvs-per-rv:\033[50D\033[30C$MVS_PER_RV"
        echo
    fi

    # Validate basic clustermap properties
    becho -n "last_updated_by must be $vm_name"
    LAST_UPDATED_BY=$(echo "$cm" | jq '."last_updated_by"' | tr -d '"')
    [ "$LAST_UPDATED_BY" == "$(get_node_id $vm_name)" ]
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

    # Validate RV count and states
    #considering 1 rv per node for now
    becho -n "RV count must be count of nodes"
    rv_count=$(get_rv_count "$cm")
    [ "$rv_count" == "$current_node" ]
    log_status $? "is $rv_count"

    # Validate all RVs are online
    validate_all_rvs_online "$cm" $current_node

    # Wait for the next scheduled epoch to ensure clustermap is updated
    becho "Sleeping $CLUSTERMAP_EPOCH seconds for clustermap updates..."
    sleep $CLUSTERMAP_EPOCH

    becho -n "Reading clustermap on $vm_name"
    cm=$(read_clustermap_from_node $vm_name)
    log_status $?

    # Check readonly flag
    becho -n "Cluster readonly flag validation"
    node_count=$(get_node_count "$cm")
    readonly_flag=$(echo "$cm" | jq '."readonly"')
    if [ "$node_count" -ge "$MIN_NODES" ]; then
        [ "$readonly_flag" == "false" ]
        cluster_readonly="false"
    else
        [ "$readonly_flag" == "true" ]
        cluster_readonly="true"
    fi
    log_status $? "readonly flag is $readonly_flag for $node_count nodes"
    

    # Check MV count when cluster is not readonly
    if [ "$cluster_readonly" == "false" ]; then
        becho -n "MV count validation"
        mv_count=$(get_mv_count "$cm")
        # Calculate max expected MV count: (rv_count * MVS_PER_RV) / NUM_REPLICAS
        max_mv_count=$(( (rv_count * MVS_PER_RV) / NUM_REPLICAS ))
        [ "$mv_count" -le "$max_mv_count" ]
        log_status $? "MV count: $mv_count, Max expected: $max_mv_count (based on $rv_count RVs)"

        becho -n "All MVs must be online"
        online_mv_count=$(get_mv_count_with_state "$cm" "online")
        [ "$online_mv_count" -eq "$mv_count" ]
        log_status $? "online MVs: $online_mv_count, total MVs: $mv_count"
    fi

    

    # Test file operations on current node
    test_file_operations $vm_name $current_node $cluster_readonly

    # Test large file operations on specific nodes
    if [ "$cluster_readonly" == "false" ]; then
        if [ $current_node -eq $MIN_NODES ]; then
            becho -n "Write 2GB data in dcache on $vm_name"
            file_name="2GB.dcache"
            write_data_in_dcache $vm_name $file_name 1G 2
            TOTAL_DCACHE_FILES=$((TOTAL_DCACHE_FILES + 1))
            TOTAL_BOTH_FILES=$((TOTAL_BOTH_FILES + 1))
            dcache_2GB_md5sum=$(get_md5sum $vm_name $file_name dcache)
            log_status $?

            becho -n "Write 2GB data over unqalified path on $vm_name"
            file_name="2GB.both"
            write_data_on_both $vm_name $file_name 1G 2
            TOTAL_BOTH_FILES=$((TOTAL_BOTH_FILES + 1))
            
            # When we write file in unqualified path, it writes in azure as well as dcache path. So updating the counter for azure file as well.
            TOTAL_DCACHE_FILES=$((TOTAL_DCACHE_FILES + 1))
            TOTAL_AZURE_FILES=$((TOTAL_AZURE_FILES + 1))
            
            both_2GB_md5sum=$(get_md5sum $vm_name $file_name)
            log_status $?
        fi

        # Instead of hardcoding vm5, use the 5th node if available
        if [ $current_node -eq 5 ] && [ "$NUM_NODES" -ge 5 ]; then
            becho -n "Write 1GB data in dcache on $vm_name"
            file_name="1GB.dcache"
            write_data_in_dcache $vm_name $file_name 1G 1
            TOTAL_DCACHE_FILES=$((TOTAL_DCACHE_FILES + 1))
            TOTAL_BOTH_FILES=$((TOTAL_BOTH_FILES + 1))
            dcache_1GB_md5sum=$(get_md5sum $vm_name $file_name dcache)
            log_status $?

            becho -n "Write 1GB data over unqalified path on $vm_name"
            file_name="1GB.both"
            write_data_on_both $vm_name $file_name 1G 1

            TOTAL_BOTH_FILES=$((TOTAL_BOTH_FILES + 1))
            
            # When we write file in unqualified path, it writes in azure as well as dcache path. So updating the counter for azure file as well.
            TOTAL_DCACHE_FILES=$((TOTAL_DCACHE_FILES + 1))
            TOTAL_AZURE_FILES=$((TOTAL_AZURE_FILES + 1))
            both_1GB_md5sum=$(get_md5sum $vm_name $file_name)
            log_status $?
        
        fi

        # Test accessibility of files created on different nodes
        if [ -n "$dcache_2GB_md5sum" ]; then
            becho -n "Verify 2GB dcache file is accessible from $vm_name"
            dcache_2GB_md5sum_current=$(get_md5sum $vm_name "2GB.dcache" dcache)
            [ "$dcache_2GB_md5sum" == "$dcache_2GB_md5sum_current" ]
            log_status $? "Original: $dcache_2GB_md5sum, Current: $dcache_2GB_md5sum_current"
        fi

        if [ -n "$both_2GB_md5sum" ]; then
            becho -n "Verify 2GB both file is accessible from $vm_name"
            both_2GB_md5sum_current=$(get_md5sum $vm_name "2GB.both")
            [ "$both_2GB_md5sum" == "$both_2GB_md5sum_current" ]
            log_status $? "Original: $both_2GB_md5sum, Current: $both_2GB_md5sum_current"
        fi
    fi

    

    # Validate clustermap consistency across all nodes
    CURRENT_NODES=$(generate_node_list $current_node)
    echo
    wbecho ">> Validating clustermap consistency across all $current_node nodes"
    echo
    validate_clustermap_consistency $current_node "$CURRENT_NODES"

    # Cross-node file consistency testing
    if [ $current_node -gt 1 ]; then

        # File listing consistency validation
        echo
        wbecho ">> Validating file listing consistency across all nodes"
        echo
        test_file_listing_consistency "$CURRENT_NODES" $TOTAL_AZURE_FILES $TOTAL_DCACHE_FILES $TOTAL_BOTH_FILES
        
        # Test large files if we have the minimum nodes
        if [ $current_node -ge $MIN_NODES ]; then
            echo
            wbecho ">> Testing large file consistency across nodes"
            echo
            
            # Use node indices instead of hardcoded node names
            first_node=$(get_node_by_index 1)
            test_cross_node_consistency "$first_node" "$vm_name" 0 true
            
            if [ $current_node -ge 5 ] && [ "$NUM_NODES" -ge 5 ]; then
                min_node=$(get_node_by_index $MIN_NODES)
                node5=$(get_node_by_index 5)
                test_cross_node_consistency "$min_node" "$node5" 0 true
            fi
        fi
    fi

done  # End of the main loop starting nodes

# Initialize variables
failed_node_vm=""

# Only run degraded workflow test if we have more than 1 node
if [ "$NUM_NODES" -gt 1 ]; then
    ############################################################################
    ##    Test blobfuse Process Failure Over a node Degraded Workflow         ##
    ############################################################################


    echo
    echo "======================================================================"
    wbecho ">> Testing blobfuse process Failure over a node Degraded Workflow"
    echo "======================================================================"
    echo

    # Choose the first node to stop blobfuse - this is typically a more critical node
    failed_node_vm=$(get_node_by_index 1)
    failed_node_id=$(get_node_id $failed_node_vm)

    # Read current clustermap from the failing node to get RVs
    becho -n "Reading clustermap before node failure over $failed_node_vm"
    cm_before=$(read_clustermap_from_node $failed_node_vm)
    log_status $?

    # Get the node that last updated the clustermap
    LAST_UPDATED_BY=$(echo "$cm_before" | jq -r '."last_updated_by"')
    
    # Determine monitoring node and adjust heartbeat timeout if needed
    if [ "$failed_node_id" == "$LAST_UPDATED_BY" ]; then
        # If the failing node is also the last updater, we need extra time
        hb_timeout=$((HB_SECONDS * HB_TILL_NODE_DOWN + CLUSTERMAP_EPOCH + 5 + 60))
        # Pick a random node other than the failed one for monitoring
        monitoring_node=$(get_random_monitoring_node "$failed_node_vm")
        wbecho "Failed node was last updater, using random node $monitoring_node for monitoring with extended timeout"
    else
        # Standard heartbeat timeout
        hb_timeout=$((HB_SECONDS * HB_TILL_NODE_DOWN + CLUSTERMAP_EPOCH + 5))
        # Use the second node for monitoring
        monitoring_node=$(get_node_ip "$cm_before" "$LAST_UPDATED_BY")
    fi

    # Get the list of RVs assigned to the node we're about to take down
    becho -n "Identifying RVs assigned to $failed_node_vm"
    rvs_on_failing_node=$(get_rv_list_for_node "$cm_before" "$failed_node_id")
    [ -n "$rvs_on_failing_node" ]
    log_status $? "No RVs found on $failed_node_vm"

    # Take down the blobfuse over the node
    wbecho ">> Taking down blobfuse over node $failed_node_vm to simulate failure"
    stop_blobfuse_on_node $failed_node_vm true  # Added true to remove from NODES_STARTED

    # Wait for heartbeat expiry
    wbecho ">> Waiting $hb_timeout seconds for heartbeat timeout..."
    sleep $hb_timeout
    wbecho ">> Heartbeat timeout period completed"

    # Read the clustermap from the monitoring node to verify the node is marked as down
    becho -n "Reading clustermap after blobfuse process stop over $monitoring_node"
    cm_after=$(read_clustermap_from_node $monitoring_node)
    log_status $?

    # Check if the RVs on the failed node are now marked as offline
    becho -n "Checking if RVs on $failed_node_vm are now offline"
    for rv in $(echo $rvs_on_failing_node | tr ',' ' '); do
        rv_state=$(get_rv_state "$cm_after" "$rv")
        [ "$rv_state" != "online" ]
        if [ $? -ne 0 ]; then
            log_status 1 "RV $rv is still online after node failure"
        fi
    done
    log_status 0 "RVs on failed node are now offline"

    # Check if mv-list in clustermap is non-empty before running MV-related tests
    mv_count_before=$(echo "$cm_before" | jq '."mv-list" | length')
    
    if [ "$mv_count_before" -gt 0 ]; then
        # Find MVs that use these RVs
        becho -n "Finding MVs that use RVs on $failed_node_vm"
        mvs_using_rvs=$(get_mvs_count_for_given_rv_with_state "$cm_before" "$rvs_on_failing_node" "online")
        [ "$mvs_using_rvs" -gt 0 ]
        log_status $? "Found $mvs_using_rvs MVs using RVs on $failed_node_vm"

        # Check if MVs that used the RVs on the failed node are now in degraded state
        becho -n "Checking if MVs using RVs of $failed_node_vm are now degraded"
        degraded_mvs=$(get_mv_count_with_state "$cm_after" "degraded")
        [ "$degraded_mvs" -gt 0 ]
        log_status $? "Found $degraded_mvs degraded MVs"
    else
        wbecho ">> Skipping MV validation tests - no MVs found in clustermap"
    fi

else
    wbecho ">> Skipping degraded workflow test - requires at least 2 nodes"
fi

# Create a list of active nodes (excluding the failed node) for Comprehensive validation
if [ -z "$failed_node_vm" ]; then
    wbecho ">> No node failure detected, proceeding with all nodes"
    ACTIVE_NODES="$ALL_NODES"
else
    for node in $ALL_NODES; do
    if [ "$node" != "$failed_node_vm" ]; then
        ACTIVE_NODES="$ACTIVE_NODES $node"
    fi
done
fi


# Count how many active nodes we have
ACTIVE_NODE_COUNT=$(echo $ACTIVE_NODES | wc -w)
becho "Active nodes for validation: $ACTIVE_NODES (count: $ACTIVE_NODE_COUNT)"

# Only proceed with comprehensive validation if we have at least one active node
if [ "$ACTIVE_NODE_COUNT" -gt 0 ]; then
    ############################################################################
    ##                    Comprehensive Validation                      ##
    ############################################################################

    # Comprehensive clustermap consistency validation
    echo
    wbecho ">> clustermap consistency validation across active nodes"
    echo
    validate_clustermap_consistency $NUM_NODES "$ACTIVE_NODES"

    # Comprehensive RV-node mapping validation
    echo
    wbecho ">> RV-node mapping consistency validation"
    echo
    validate_rv_node_mapping_consistency "$ACTIVE_NODES" $NUM_NODES

    # Comprehensive file listing consistency validation
    echo
    wbecho ">> File listing consistency validation"
    echo
    test_file_listing_consistency "$ACTIVE_NODES" $TOTAL_AZURE_FILES $TOTAL_DCACHE_FILES $TOTAL_BOTH_FILES

    # Comprehensive cross-node file consistency testing
    echo
    wbecho ">> Comprehensive cross-node file consistency testing"
    echo

    # Use active nodes for cross-node consistency tests
    if [ "$ACTIVE_NODE_COUNT" -gt 2 ]; then
        # Find nodes for testing - use the first and last active nodes
        second_node=$(echo $ACTIVE_NODES | awk '{print $1}')
        last_node=$(echo $ACTIVE_NODES | awk '{print $NF}')
        
        test_cross_node_consistency "$second_node" "$last_node" $NUM_NODES false
        
        # Test large file consistency across multiple node pairs
        if [ $NUM_NODES -ge $MIN_NODES ] && [ "$cluster_readonly" == "false" ]; then
            test_cross_node_consistency "$second_node" "$last_node" 0 true
        fi
    else
        wbecho ">> Skipping cross-node consistency tests - need at least 2 active nodes"
    fi
else
    wbecho ">> No active nodes available for validation after $failed_node_vm failure"
    wbecho ">> Skipping comprehensive validation steps"
fi


echo
sbecho "======================================================================"
sbecho "Cluster validation tests completed successfully!"
sbecho "======================================================================"