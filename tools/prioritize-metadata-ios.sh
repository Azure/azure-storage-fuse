#!/bin/bash

#
# This script uses tc to create a prio qdisc attached as the root qdisc
# of eth0. This prio qdisc has teo priority bands (or classes)"
#
# prio 0: For all metadata traffic (to por 443).
# prio 1: For all other traffic, including the bulk GetChunk/PutChunk traffic.
#
# We set the priomap to all 1's which means by default any traffic that is not
# classified goes to prio 1 (the lower priority) class. This way we can control
# what traffic goes to prio 0 (the higher priority).
#
# To go back to default qdisc, simply delete using:
#
# tc qdisc delete dev eth0 root
#

set_prio()
{
        sudo tc qdisc delete dev eth0 root 2>/dev/null
        sudo tc qdisc add dev eth0 root handle 1: prio bands 2 priomap 1 1 1 1 1 1 1 1 1 1 1 1 1 1 1 1

        sudo tc qdisc add dev eth0 parent 1:1 handle 10: pfifo limit 2000
        sudo tc qdisc add dev eth0 parent 1:2 handle 20: pfifo limit 10000

        # port 443 is prioritized.
        sudo tc filter add dev eth0 protocol ip parent 1: prio 1 u32 match ip dport 443 0xffff classid 1:1
        # < 256 bytes packes are also prioritized.
        sudo tc filter add dev eth0 protocol ip parent 1: prio 10 u32 match u16 0x0000 0xff00 at 2 classid 1:1
        # also ping
        sudo tc filter add dev eth0 protocol ip parent 1: prio 2 u32 match ip protocol 1 0xff classid 1:1

        sudo tc qdisc show dev eth0
}

unset_prio()
{
        sudo tc qdisc delete dev eth0 root
        sudo tc qdisc show dev eth0
}

if [ "$1" == "set" ]; then
        set_prio
else
        unset_prio
fi
