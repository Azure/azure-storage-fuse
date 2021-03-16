#This ia manual test file in Python for testing if 
#blobfuse blocks PUT operations despite calls to azs_flush typically trigerred during file.close() 
#when there are no writes to a file.
#Multiple file flushes here is triggerred by fork a process.
#prerequisites : mount dir is /home/nara/mntblobfuse/

import time
from multiprocessing import Process
def workload_process(name):
    print(f"process {name} started.")
    time.sleep(10)
    print(f"process {name} ended.")
def start_process(name):
    p = Process(
        target=workload_process,
        args=(name,),
    )
    p.daemon = True
    p.start()
def main():
    with open("/home/nara/mntblobfuse/user_script_log.txt", "w") as log:
        log.write("test started.")
        group_index=1
        for proc_index in range(50):
            process_name = f"proc_{group_index}_{proc_index}"
            start_process(process_name)
        time.sleep(60)
        log.write("test ended.")
if __name__ == "__main__":
    main()