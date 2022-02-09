#!/bin/bash

rm -rf tmp.lst
rm -rf missing_log.lst

for i in $(find . -name "*.go"  | grep -v "_test.go"| grep -v "manual_scripts"); do
    #echo $i
    grep -A 7 "err :*= " $i > tmp.lst
    err_cnt=`grep -c "err :*= " tmp.lst`
    log_cnt=`grep -c "log." tmp.lst`

    if [[ $err_cnt -ne $log_cnt ]]
    then
        echo "------------------------------------------------------------" >> missing_log.lst
        echo "Logs not present for all errors in $i ($err_cnt : $log_cnt)" >> missing_log.lst
        grep -A 5 "err :*= " $i >> missing_log.lst
    #else 
    #    echo "Logs present for all errors in $i"
    fi
done

rm -rf tmp.lst
