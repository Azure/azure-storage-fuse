#!/bin/bash

#==============================================================================
#title           : notices_fix.sh
#description     : Using go.sum file generates third-party notices for the project
#author		     : Vikas Bhansali (vibhansa@microsoft.com)
#date            : 02-Oct-2021
#usage		     : sh notices_fix.sh
#==============================================================================


# File to hold dependency list from go.sum file
dependency_list="./dependencies.lst"
output_file="./NOTICE"

# Function to create header for notices file
function dump_header
{
    # Dump static header to NOTICE file
    echo "--------------------- START OF THIRD PARTY NOTICE -----------------------------

    NOTICES AND INFORMATION
    Do Not Translate or Localize

    This software incorporates material from third parties. Microsoft makes certain
    open source code available at https://3rdpartysource.microsoft.com, or you may
    send a check or money order for US $5.00, including the product name, the open
    source component name, and version number, to:

    Source Code Compliance Team
    Microsoft Corporation
    One Microsoft Way
    Redmond, WA 98052
    USA

    Notwithstanding any other terms, you may reverse engineer this software to the
    extent required to debug changes to any libraries licensed under the GNU Lesser
    General Public License.

    Third Party Programs: The software may include third party programs that Microsoft,
    not the third party, licenses to you under this agreement. Notices, if any, for the
    third party programs are included for your information only.
    ---------------------------------------------------------------------------------------
    " > $output_file
}

function dump_footer
{
    echo -ne "--------------------- END OF THIRD PARTY NOTICE --------------------------------\n" >> $output_file
}

function append_lic_to_notice
{
    echo >> $output_file
    echo -e "\n\n" >> $output_file
    echo "****************************************************************************" >> $output_file
    echo -e "\n============================================================================" >> $output_file
    echo -e ">>>" $1 >> $output_file
    echo -e "==============================================================================" >> $output_file
    
    echo >> $output_file
    cat lic.tmp >> $output_file
    rm -rf lic.tmp 
}

# Function to download the file and add it to Notice file with formatting
function download_and_dump
{
    #echo "Downloading lic for $1 from $2"
    # Cleanup old tmp file
    rm -rf lic.tmp

    # Fetch the raw license file
    wget -q -O lic.tmp $2

    if [[ $? -eq 0 ]]
    then
        append_lic_to_notice $1
        return 0
    fi

    return -1
}

function try_differ_names()
{
    # Try lic file without any extension
    download_and_dump $1 $2

    if [[ $? -ne 0 ]]
    then
        # Try with .txt extension
        download_and_dump $line $lic_path.txt
    fi

    if [[ $? -ne 0 ]]
    then
        # Try with .md extension
        download_and_dump $line $lic_path.md
    fi

    return $?
}

function download_notice
{
    line=$1

    case $line in
    *go-autorest/*)
        # There are multiple entries with this for each of its subfolder so download only once
        if [[ $autorest_done -eq 0 ]]
        then
            lic_path="https://raw.githubusercontent.com/Azure/go-autorest/master/LICENSE"

            download_and_dump "github.com/Azure/go-autorest/autorest" $lic_path
            if [[ $? -ne 0 ]]
            then
                # This might need manual intervention
                echo "Failed to get LICENSE from : AutoRest"
            else 
                autorest_done=1
            fi
        fi
        echo -ne "." ;;

    github.com*)
        # Try standard lic path first to get the info with 'master' branch
        lic_path="https://raw.githubusercontent.com/"`echo $line | cut -d "/" --complement -f1`"/master/LICENSE"
        
        try_differ_names $line $lic_path
        
        # If basic name fails try removing subfolders and search outside
        if [[ $? -ne 0 ]]
        then
            lic_path="https://raw.githubusercontent.com/"`echo $line | cut -d "/" -f2-3`"/master/LICENSE"
            try_differ_names $line $lic_path
        fi

        # Try searching if there is main branch instead of master
        if [[ $? -ne 0 ]]
        then
            lic_path="https://raw.githubusercontent.com/"`echo $line |  cut -d "/" --complement -f1`"/main/LICENSE"
            try_differ_names $line $lic_path
        fi

        # If basic name fails try removing subfolders and search outside for main branch
        if [[ $? -ne 0 ]]
        then
            lic_path="https://raw.githubusercontent.com/"`echo $line | cut -d "/" -f2-3`"/main/LICENSE"
            try_differ_names $line $lic_path
        fi

        # Some have COPYING file for license
        if [[ $? -ne 0 ]]
        then
            lic_path="https://raw.githubusercontent.com/"`echo $line | cut -d "/" --complement -f1`"/master/COPYING"
            download_and_dump $line $lic_path
        fi

        # Camel case name search in master branch
        if [[ $? -ne 0 ]]
        then
            lic_path="https://raw.githubusercontent.com/"`echo $line | cut -d "/" -f2-3`"/master/License"
            try_differ_names $line $lic_path
        fi

        # Camel case name search in main branch
        if [[ $? -ne 0 ]]
        then
            lic_path="https://raw.githubusercontent.com/"`echo $line | cut -d "/" -f2-3`"/main/License"
            try_differ_names $line $lic_path
        fi

        if [[ $? -ne 0 ]]
        then
            # This might need manual intervention
            echo "Failed to get LICENSE from : $line"
        fi
        echo -ne "." ;;

    *etcd.io/etcd*)
        # There are multiple entries with this for each of its subfolder so download only once
        if [[ $etcd_done -eq 0 ]]
        then
            lic_path="https://raw.githubusercontent.com/etcd-io/etcd/main/LICENSE"

            download_and_dump "go.etcd.io/etcd" $lic_path
            if [[ $? -ne 0 ]]
            then
                # This might need manual intervention
                echo "Failed to get LICENSE from : etcd.id"
            else 
                etcd_done=1
            fi
        fi
        echo -ne "." ;;


    *golang.org/x* | *rsc.io/* | *cloud.google.com/* | *google.golang.org/* | *go.uber.org/* | *go.opencensus.io* | *go.opentelemetry.io/*) 
        #echo ">>> " $line
        # Get the contents of this package
        wget -q -O lic.tmp "https://pkg.go.dev/$line?tab=licenses"
        
        if [[ $? -ne 0 ]]
        then
            # This might need manual intervention
            echo "Failed to get LICENSE from : $line"
        else
            # This will be html output so filter only license content
            sed -n '/License-contents/,/\/pre\>/p' lic.tmp > lic1.tmp
            head -1 lic1.tmp | grep "Copyright" | cut -d ">" -f 2 > lic.tmp
            sed '1d;$d' lic1.tmp >> lic.tmp

            # now dump it to our notice file
            append_lic_to_notice $line
        fi
        echo -ne "." ;;

    *gopkg.in/ini.v1*)
        wget -q -O lic.tmp "https://raw.githubusercontent.com/go-ini/ini/v1.63.2/LICENSE"
        if [[ $? -ne 0 ]]
        then
            # This might need manual intervention
            echo "Failed to get LICENSE from : $line"
        else
            append_lic_to_notice $line
        fi

        echo -ne "." ;;

    *gopkg.in/*)
        #https://raw.githubusercontent.com/go-yaml/yaml/v3/LICENSE
        pkg=`echo $line | cut -d "/" -f 2 | cut -d "." -f 1`
        ver=`echo $line | cut -d "/" -f 2 | cut -d "." -f 2`
        wget -q -O lic.tmp "https://raw.githubusercontent.com/go-$pkg/$pkg/$ver/LICENSE"
        if [[ $? -ne 0 ]]
        then
            # This might need manual intervention
            echo "Failed to get LICENSE from : $line"
        else
            append_lic_to_notice $line
        fi

        echo -ne "." ;;

    *dmitri.shuralyov.com*)
        #dmitri.shuralyov.com/gpu/mtl    
        # Get the contents of this package
        wget -q -O lic.tmp "https://$line\$file/LICENSE"
        if [[ $? -ne 0 ]]
        then
            # This might need manual intervention
            echo "Failed to get LICENSE from : $line"
        else
            append_lic_to_notice $line
        fi
        echo -ne "." ;;

    *honnef.co/go/tools*)
        # Get the contents of this package
        wget -q -O lic.tmp "https://raw.githubusercontent.com/dominikh/go-tools/master/LICENSE"
        if [[ $? -ne 0 ]]
        then
            # This might need manual intervention
            echo "Failed to get LICENSE from : $line"
        else
            append_lic_to_notice $line
        fi
        echo -ne "." ;;

    *)
        echo "Others: " $line;;
    esac
}

function generate_notices
{
    ret=0
    while IFS= read -r line; do
        case $line in
        *go-autorest/*)
            if grep -q ">>> github.com/Azure/go-autorest/autorest" $output_file
            then
                echo -ne "." 
            else
               #echo "Missing $line in old file" 
               download_notice $line
            fi
            echo -ne "." ;;

        *etcd.io/etcd*)
            if grep -q ">>> go.etcd.io/etcd" $output_file
            then
                echo -ne "." 
            else
               #echo "Missing $line in old file" 
               download_notice $line
            fi
            echo -ne "." ;;
            
        *)
            if grep -q ">>> $line" $output_file
            then
                echo -ne "."
            else
                #echo "Missing $line in old file"
                download_notice $line
                ret=1
            fi
            echo -ne "." ;;
        esac
    done < $dependency_list

    return $ret
}


# Create temp directory for working on this 
rm -rf ./notice_tmp
mkdir ./notice_tmp/
chmod 777 ./notice_tmp/
cd ./notice_tmp/


# From go.sum file create unique list of dependencies we have
echo "Searching for dependencies"
cat ../go.sum | cut -d " " -f 1 | sort -u > $dependency_list


echo "Populating Notices"
# Check if notice.txt file exists or not
if [ -e ../$output_file ]
then
    # file is already there so copy that to temp folder
    # Ignore the file footer while making this copy
    echo "File exists check for new dependencies only"
    head -n -1 ../$output_file > $output_file
else
    # Main code to call the respective methods
    echo "File does not exists, start from scratch"
    dump_header
fi

# Generate notices in a temp file now
generate_notices


# Generate footer for the file
if [[ $? -ne 0 ]]
then
    # File is modified make space for fotter
    echo -e "\n" >> $output_file
fi
dump_footer


echo "Comparing missing dependencies"
# Compare the input list and notice file for final consolidation
grep ">>>" $output_file | cut -d " " -f 2 > notice.lst
diff $dependency_list notice.lst | grep -v "go-autorest" | grep -v "go.etcd.io"

# Delete the temp directory
cp $output_file ../NOTICE
cd -
rm -rf ./notice_tmp/

echo "NOTICE updated..."


