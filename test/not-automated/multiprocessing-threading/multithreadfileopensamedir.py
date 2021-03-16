# This is a multithreaded test run as a threadpool by a multiprocessing
# codeflow that maximizes the CPU processes. 
# The purpose of this test is detect file reading deadlocks during multiprocessing and multithreading.
# Currently this test has to be run manually for verification
# prerequisites : mount dir is /home/nara/mntblobfuse/
# prerequisite 2: file_cache_timeout_in_seconds=0
import time
import os
from threading import Thread
import multiprocessing
from multiprocessing import Process
from itertools import product
from contextlib import contextmanager
import random

number_of_processes = multiprocessing.cpu_count()
number_of_threads_in_process = 40   # some constant, keep it high to be able to repro the error
# make sure the following files exist in whichever blob container you are trying to mount
file_list = ["/home/nara/mntblobfuse/avi-file.avi",
"/home/nara/mntblobfuse/avi-file_copy1.avi",
"/home/nara/mntblobfuse/avi-file_copy2.avi",
"/home/nara/mntblobfuse/OTT file.ott",
"/home/nara/mntblobfuse/OTT file_copy1.ott",
"/home/nara/mntblobfuse/OTT file_copy2.ott"]

def workload_thread(name):    
    rfile = random.choice(file_list)
    #print(f"thread {name} started.")
    fi = open(rfile, "rb")    
    print(f"Thread: {name} fi: {fi.fileno()} size: {os.fstat(fi.fileno()).st_size}")
    firstbytes = fi.read(16)
    print(f"Thread: {name} fi: {fi.fileno()} firstbytes: {firstbytes}")
    fi.close()
    print(f"thread {name} ended.")
def start_workload_thread(name):
    print (f"inside start_workload_thread {name}")
    for t_index in range(number_of_threads_in_process):
        t_name = f"t_{name}_{t_index}"
        start_thread(t_name)
def start_thread(name):
    print (f"inside start_thread {name}")
    t = Thread(
        target=workload_thread,
        args=(name,),
    )
    t.daemon = True
    t.start()

@contextmanager
def poolcontext(*args, **kwargs):
    pool = multiprocessing.Pool(*args, **kwargs)
    yield pool    
    time.sleep(180)
    pool.terminate()

def main():
    names = ['proc1', 'proc2', 'proc3', 'proc4', 'proc5']
    with poolcontext(number_of_processes) as pool:
        results = pool.map(start_workload_thread, names)
    
if __name__ == "__main__":
    main()