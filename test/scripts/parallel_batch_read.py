
import os
import threading
import hashlib
import datetime
import random
# Directory containing the files

mountroot = "/tmp/mntpoint1"
mountpath = "/smallfiles/createfiles"
data_dir = mountroot + mountpath

# Number of files to read in each batch
batch_size = 10000

# block size of the read requests
blockSize = 8 * 1024 * 1024

def read_batch(batch):
    for filename in batch:
        open_start_time = datetime.datetime.now()
        print("filename: "+ filename + "PID: " + str(os.getpid()) + "TID: "+ str(threading.get_native_id()))

        f1 = ""
        with open(os.path.join(data_dir, filename), "rb") as f:
            open_end_time = datetime.datetime.now() - open_start_time
            
            start_time = datetime.datetime.now()
            size = 0
            cnt = 0
            while True:
                cnt += 1
                data = f.read(blockSize)
                
                # Needed for data validation, increases time taken so remove if not needed
                f1+=str(data)
                size += len(data)
                if not data:
                    break
        
        end_time = datetime.datetime.now()
        print(filename + ":readtime:" + str(end_time - start_time) + ":opentime:" + str(open_end_time)+ ":openstarttime:" + str(open_start_time) + "size: " + str(size))
        f.close()
        
        hash = hashlib.md5(f1.encode()).hexdigest()
        print(hash)

# Number of iterations to run
count = 1 
while count>0:
    count -= 1
    
    # list files and calculate list time
    start_time = datetime.datetime.now()
    filenames = os.listdir(data_dir)

    end_time = datetime.datetime.now()
    print("FileListTime: " + str(end_time - start_time))
    
    # for local testing use by name
    #filenames = ["sample31", "sample31"]
    
    random.shuffle(filenames)

    # create batches and threads
    batches = [filenames[i:i+batch_size] for i in range(0, len(filenames), batch_size)]
    print(len(batches))

    threads = []
    for batch in batches:
        t = threading.Thread(target=read_batch, args=(batch,))
        threads.append(t)

    start_time = datetime.datetime.now()

    for t in threads:
        t.start()

    for t in threads:
        t.join()

    end_time = datetime.datetime.now()
    print("FileOpenReadCloseTime: " + end_time - start_time)