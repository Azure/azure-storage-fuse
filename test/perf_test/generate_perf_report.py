# Python program to read
# json file


import json
import argparse
import sys
import os

def compare_numbers(job_one, job_two, metrics_list, log_file):
    f = open(log_file)
    data = json.load(f)
    for i in metrics_list:
        metric_value = ((data[job_one][i]/data[job_two][i])*100)-100
        if metric_value < 0:
            sys.stdout.write('{} has regressed - there is a perf regression of {}\n'.format( i, metric_value))
        if metric_value >= 0:
            sys.stdout.write('{} has a perf improvement of {}%\n'.format(i, metric_value))
    f.close()


if __name__ == "__main__":
    # parse argument
    parser = argparse.ArgumentParser("compare performance")
    parser.add_argument('-j1', '--job1', default='main', help='name of the first job', required=False)
    parser.add_argument('-j2', '--job2', default='binary', help='name of the second job', required=False)
    parser.add_argument('-m','--metrics', nargs='+', help='metrics to compare from log file', required=True)
    parser.add_argument('-lf',
                        '--log',
                        default="./blobfuse2-perf.log",
                        help='path of log file', 
                        required=False)
    args = vars(parser.parse_args())
    log_file = args['log']
    job_one_name = args['job1']
    job_two_name = args['job2']
    metrics_list = args['metrics']

    compare_numbers(job_one_name, job_two_name, metrics_list, log_file)
    