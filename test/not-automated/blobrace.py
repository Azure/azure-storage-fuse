import os
import sys
import time
from multiprocessing import Pool

def noop():
    time.sleep(0.1)
    return None

def read_blob(paths):
    fh = open(path, 'r')
    fh.seek(0)
    data = fh.readline()
    return len(data)

WORKERS=100

def test_blob(path):
    pool = Pool(processes=WORKERS)
    tasks = [ pool.apply_async(noop, ()) for i in range(WORKERS) ]
    [ t.get() for t in tasks ]
    tasks = [ pool.apply_async(read_blob, (path, )) for i in range(WORKERS) ]
    lengths = [ t.get() for t in tasks ]
    print(path, lengths)
    assert 0 not in lengths

if __name__ == '__main__':
    for path in sys.argv[1:]:
        test_blob(path)