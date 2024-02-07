import time 
import os 
import sys 

mountpath = sys.argv[1] 
size = sys.argv[2] 
tool = sys.argv[3] 
# Write some data 
blockSize = 8 * 1024 * 1024 
fileSize = int(size) * (1 * 1024 *1024) 
bytes_return = 0 
data = os.urandom(blockSize)     
t1 = time.time() 
fd = open(os.path.join(mountpath, "pythonWrite_"+size+".data"), "wb") 
t2 = time.time() 
while bytes_return <= fileSize: 
    data_byte = fd.write(data) 
    bytes_return += data_byte 
t3 = time.time() 
fd.close() 
t4 = time.time() 

open_time = t2 - t1 
close_time = t4 - t3 
write_time = t3 - t2 
total_time = t4 - t1 
write_mbps = ((bytes_return/write_time) * 8)/(1024 * 1024) 
total_mbps = ((bytes_return/total_time) * 8)/(1024 * 1024) 
print(tool, size, open_time, write_time, close_time, total_time, write_mbps, total_mbps) 