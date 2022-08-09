#!/bin/bash

overall_check() {
    cvg=`tail -1 ./blobfuse2_func_cover.rpt | cut -d ")" -f2 | sed -e 's/^[[:space:]]*//' | cut -d "%" -f1`
    cvgVal=`expr $cvg`
    echo $cvgVal
    if [ 1 -eq "$(echo "${cvgVal} < 80" | bc)" ]
    then
        echo "Code coverage below 80%"
        exit 1
    fi
    echo "Code coverage success"
}

file_check() {
    flag=0

    for i in `grep "value=\"file" ./blobfuse2_coverage.html | cut -d ">" -f2 | cut -d "<" -f1 | sed -e "s/ //g"`
    do 
        fileName=`echo $i | cut -d "(" -f1`
        percent=`echo $i | cut -d "(" -f2 | cut -d "%" -f1`
        percentValue=`expr $percent`
        if [ 1 -eq "$(echo "${percentValue} < 80" | bc)" ]
        then
            flag=1
            echo $fileName" : "$percentValue
        fi
    done
    if [ $flag -eq 1 ]
    then
        echo "Code coverage below 80%"
        exit 1
    fi
    echo "Code coverage success"
}

if [[ $1 == "file" ]]
then
    file_check
else
    overall_check
fi