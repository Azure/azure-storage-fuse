import time
import os
import sys
import json
 
mountpath = sys.argv[1]
size = sys.argv[2]
 
blockSize = 8 * 1024 * 1024
fileSize = int(size) * (1024 * 1024 * 1024)
bytes_written = 0
 
data = os.urandom(blockSize)
 
t1 = time.time()
fd = open(os.path.join(mountpath, "application_"+size+".data"), "wb")
t2 = time.time()
 
while bytes_written <= fileSize:
    data_byte = fd.write(data)
    bytes_written += data_byte
 
t3 = time.time()
fd.close()
t4 = time.time()
 
open_time = t2 - t1
close_time = t4 - t3
write_time = t3 - t2
total_time = t4 - t1

write_mbps = ((bytes_written/write_time) * 8)/(1024 * 1024)
total_mbps = ((bytes_written/total_time) * 8)/(1024 * 1024)
 
print(json.dumps({"name": "write_" + size + "GB", "open_time": open_time, "write_time": write_time, "close_time": close_time, "total_time": total_time, "write_mbps": write_mbps, "speed": total_mbps, "unit": "MiB/s"}))