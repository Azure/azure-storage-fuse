import multiprocessing
from multiprocessing import Pool
import threading
import time
import os

task_list = [ ["/home/vikas/blob_mnt/mount.sh", 251],
              ["/home/vikas/blob_mnt/blobfuse.log1", 39834954]
            ]

file_size_0 = 0
file_size_less = 0
file_size_correct = 0

def worker(num):
    global file_size_0, file_size_less, file_size_correct

    ret_val = 3
    file_size = os.stat(task_list[num][0])
    if file_size.st_size == 0 :
        file_size_0 = file_size_0 + 1
        ret_val = 1
        for i in range(10) :
            file_size = os.stat(task_list[num][0])
            if file_size == 0:
                print(str(num) + " : File size is stil 0")
                
    elif file_size.st_size != task_list[num][1] :
        file_size_less = file_size_less + 1
        ret_val = 2
    else :
        file_size_correct = file_size_correct + 1
        ret_val = 3

    fh = open(task_list[num][0], 'r')
    fh.seek(num*100)
    data = fh.readline()
    fh.close()

    return ret_val

def start_process() :
    global file_size_0, file_size_less, file_size_correct

    jobs = [] # list of jobs
    jobs_num = 4000 # number of workers

    for i in range(jobs_num):
        p1 = threading.Thread(target=worker, args=(0,))
        jobs.append(p1)
        p1.start() 

    jobs_num = 1000
    for i in range(jobs_num):
        p1 = threading.Thread(target=worker, args=(1,))
        jobs.append(p1)
        p1.start() 
        #time.sleep(0.005)

    for j in jobs :
        j.join()

    print("File size found 0     : " + str(file_size_0) )
    print("File size found less  : " + str(file_size_less) )
    print("File size found nice  : " + str(file_size_correct) )

if __name__ == '__main__':
    start_process()
