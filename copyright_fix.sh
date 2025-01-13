#!/bin/bash

# Update LICENSE and component.template files with correct year (Eg: "Copyright © 2020-2025" to "Copyright © 2020-2026") and run the script ./copyright.sh
currYear=`date +"%Y"`
searchStr="Copyright ©"
copyLine=`grep -h $searchStr LICENSE`

if [[ "$1" == "replace" ]]
then 
    for i in $(find -name \*.go); do
        result=$(grep "$searchStr" $i)
        if [ $? -ne 1 ]
        then
            echo "Replacing in $i"
            result=$(grep "+build" $i)
            if [ $? -ne 1 ]
            then
                sed -i -e '5,32{R LICENSE' -e 'd}' $i
            else
                sed -i -e '2,31{R LICENSE' -e 'd}' $i
            fi
        fi
    done
else
    for i in $(find -name \*.go); do
        result=$(grep "$searchStr" $i)
        if [ $? -eq 1 ]
        then
            echo "Adding Copyright to $i"
            result=$(grep "+build" $i)
            if [ $? -ne 1 ]
            then
                echo $result  > __temp__
                echo -n >> __temp__
                echo "/*" >> __temp__
                cat LICENSE >> __temp__
                echo -e "*/" >> __temp__
                tail -n+2 $i >> __temp__
            else
                echo "/*" > __temp__
                cat LICENSE >> __temp__
                echo -e "*/\n" >> __temp__
                cat $i >> __temp__
            fi
            mv __temp__ $i
        else
            currYear_found=$(echo $result | grep $currYear)
            if [ $? -eq 1 ]
            then
                echo "Updating Copyright in $i"
                sed -i "/$searchStr/c\\$copyLine" $i
            fi
        fi
    done
fi