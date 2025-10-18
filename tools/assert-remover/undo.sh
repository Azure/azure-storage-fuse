#!/bin/bash

# List of files to undo.
undo_files=$(find . -name "*.orig.withasserts")

for file in $undo_files; do
        orig_file=$(dirname $file)/$(basename $file ".orig.withasserts")
	if [ "$DEBUG" = "1" ]; then
		mv -vf $file $orig_file
	else
		mv -f $file $orig_file
	fi
done
