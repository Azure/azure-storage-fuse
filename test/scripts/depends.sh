
#!/bin/bash

go mod graph > graph.lst
echo "" > graph.tmp

level=0
getSource()
{
    EMPTY="$(printf '%*s' $level)"
    let "space=level * 4"
    EMPTY="$(printf '%*s' $space)"
    echo "$EMPTY|_" $1

    if grep -q $1 graph.tmp
    then
        # echo "$EMPTY >> " $lib " : Already done" 
        return
    fi

    echo $1 >> graph.tmp

    let "level=level + 1"
    grep " $1" graph.lst | cut -d " " -f 1 | cut -d "@" -f1 | sort -u| while read -r lib ; do
        if [ "$1" != "$lib" ]
        then
            getSource $lib
        # else
            # echo "$lib imported from its own previous verisons"
        fi
    done
    let "level=level - 1"
}

getSource $1

