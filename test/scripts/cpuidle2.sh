#!/bin/bash 

# Monitor the idle CPU using top 
average_usage=0 
num_samples=0  #adjust as needed 

# Function to calculate average CPU usage and cleanup 
calculate_average_and_cleanup() { 
if [ "$num_samples" -gt 0 ]; then 
        average_usage=$((average_usage / num_samples)) 
        echo "$average_usage%" > nohup.out 
    else 
        echo "No samples collected. Exiting." > nohup.out 
    fi 
    exit 
} 

# Set trap to call the cleanup function on script termination 
trap 'calculate_average_and_cleanup' EXIT 

while true;  
do 
    cpu_usage=$(top -bn1 | grep '%Cpu' | tail -1 | grep -P '(....|...) id,' | awk –v '{print int((100 - $8))}') 
    average_usage=$((average_usage + cpu_usage)) 
    num_samples=$((num_samples + 1)) 
    sleep 1 
done 