#!/bin/bash

currYear=`date +"%Y"`
searchStr="Copyright ©"
copyLine=`grep -h $searchStr LICENSE`

if [[ "$1" == "replace" ]]
then 
    for i in $(find -name \*.go | grep -v ./test/ | grep -v main_test.go); do
        result=$(grep "$searchStr" $i)
        if [ $? -ne 1 ]
        then
            echo "Replacing in $i"
            result=$(grep "+build !authtest" $i)
            if [ $? -ne 1 ]
            then
                sed -i -e '3,32{R LICENSE' -e 'd}' $i
            else
                sed -i -e '2,31{R LICENSE' -e 'd}' $i
            fi
        fi
    done
else
    for i in $(find -name \*.go); do
        if [[ $i == *"_test.go"* ]]; then
            echo "Ignoring Test Script : $i"
        else
            result=$(grep "$searchStr" $i)
            if [ $? -eq 1 ]
            then
                echo "Adding Copyright to $i"
                echo "/*" > __temp__
                cat LICENSE >> __temp__
                echo -e "*/\n\n" >> __temp__
                cat $i >> __temp__
                mv __temp__ $i
            else
                currYear_found=$(echo $result | grep $currYear)
                if [ $? -eq 1 ]
                then
                    echo "Updating Copyright in $i"
                    sed -i "/$searchStr/c\\$copyLine" $i
                fi
            fi
        fi
    done
fi