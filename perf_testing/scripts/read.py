import time
import os
import sys
import json
 
mountpath = sys.argv[1]
size = sys.argv[2]
 
blockSize = 8 * 1024 * 1024
fileSize = int(size) * (1024 * 1024 * 1024)
bytes_read = 0
 
t1 = time.time()
fd = open(os.path.join(mountpath, "application_"+size+".data"), "rb")
t2 = time.time()
 
while bytes_read <= fileSize:
    data_byte = fd.read(blockSize) 
    bytes_read += len(data_byte) 
 
t3 = time.time()
fd.close()
t4 = time.time()
 
open_time = t2 - t1 
close_time = t4 - t3 
read_time = t3 - t2 
total_time = t4 - t1 

read_mbps = ((bytes_read/read_time) * 8)/(1024 * 1024) 
total_mbps = ((bytes_read/total_time) * 8)/(1024 * 1024) 

print(json.dumps({"name": "read_" + size + "GB", "open_time": open_time, "read_time": read_time, "close_time": close_time, "total_time": total_time, "read_mbps": read_mbps, "speed": total_mbps, "unit": "MiB/s"}))