#!/bin/bash
echo "Blobfuse2 Component Generator..." 

# Create component folder and file
comp_name=`echo ${1}`
comp_name_C=`echo ${1^} | sed -r 's/\_./\U&/g' | sed -r 's/_//g'`
comp_path="./component/$comp_name"
comp_file=$comp_path/$comp_name.go 

if [ -d  $comp_path ] 
then
    echo "Component already exists. Try some other name"
    exit
else
    echo "Creating directory $comp_path"
    mkdir ./component/$comp_name
fi

# Copy template file and create a .go file for component
cat ./internal/component.template > $comp_file 
sed -i "s|<component>|$comp_name|g" $comp_file 
sed -i "s|<component_C>|$comp_name_C|g" $comp_file 

echo "Blobfuse2 Component Generated : " $comp_file

./cmd/importGenerator.sh
echo "Component generated successfully"
