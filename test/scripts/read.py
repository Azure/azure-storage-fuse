import os 
import sys 
import time

mountpath = sys.argv[1] 
size = sys.argv[2] 
tool = sys.argv[3] 

# Read some data 
blockSize = 8 * 1024 * 1024 
bytesRead = 0 
t1 = time.time() 
fd = open(os.path.join(mountpath, "pythonWrite_"+size+".data"), "rb") 
t2 = time.time() 
data_byte = fd.read(blockSize) 
bytes_read = len(data_byte) 

while data_byte: 
    data_byte = fd.read(blockSize) 
    bytes_read += len(data_byte) 
t3 = time.time() 
fd.close() 
t4 = time.time() 
  
open_time = t2 - t1 
close_time = t4 - t3 
read_time = t3 - t2 
total_time = t4 - t1 
write_mbps = ((bytes_read/read_time) * 8)/(1024 * 1024) 
total_mbps = ((bytes_read/total_time) * 8)/(1024 * 1024) 
print(tool, size, open_time, read_time, close_time, total_time, write_mbps, total_mbps) 

 