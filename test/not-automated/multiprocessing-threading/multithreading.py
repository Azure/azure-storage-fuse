from threading import Thread
import os

def threaded_function():
    top = "/home/nara/mntblobfuse/128/wiki_pretrain/"
    for folder in os.listdir(top):
        folder_orfile = os.path.join(top, folder)
        if os.path.isdir(folder_orfile):
            for f in os.listdir(folder_orfile):
                # im = np.load(os.path.join(folder_full, f), allow_pickle=True)
                if os.path.isdir(os.path.join(folder_orfile, f)):
                    parentfolder = os.path.join(folder_orfile, f)
                    for j in os.listdir(parentfolder):
                        if os.path.isdir(os.path.join(parentfolder, j)):
                            subparentfolder = os.path.join(parentfolder, j)
                            for k in os.listdir(subparentfolder):
                                if os.path.isfile(k):
                                    with open(os.path.join(parentfolder, j)):
                                        print(k)
                        else:
                            with open(os.path.join(parentfolder, j)):
                                print(j)
                else:
                    with open(os.path.join(folder_orfile, f)):
                        print(f)
        else:
            with open(folder_orfile):
                        print(folder_orfile)

if __name__ == "__main__":
    threads = []
    for i in range(5):
        thread = Thread(target=threaded_function)
        threads.append(thread)
        thread.start()
    for thread in threads:
        thread.join()
    print("Thread Exiting...")