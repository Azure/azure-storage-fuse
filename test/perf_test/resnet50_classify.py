import os
import sys
import time
import json
import argparse
import numpy as np
from multiprocessing import Pool
from tensorflow.keras.applications import resnet50
from tensorflow.keras.preprocessing.image import load_img
from tensorflow.keras.preprocessing.image import img_to_array
from tensorflow.keras.applications.imagenet_utils import decode_predictions

# we're not using any GPUs
os.environ["CUDA_DEVICE_ORDER"] = "PCI_BUS_ID"   # see issue #15
os.environ["CUDA_VISIBLE_DEVICES"] = ""


def classify_images(images):
    # we need to load the model within the process since we can't share a model across processes
    resnet_model = resnet50.ResNet50(weights='imagenet')
    
    tic = time.time()
    sys.stdout.write('starting to process {} images in this thread at time: {}\n'.format(len(images), time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(tic))))
    
    for filename in images:
        # load image
        original = load_img(filename, target_size=(224, 224))
        # transform image
        numpy_image = img_to_array(original)
        image_batch = np.expand_dims(numpy_image, axis=0)
        processed_image = resnet50.preprocess_input(image_batch)
        # predict
        predictions = resnet_model.predict(processed_image)


def chunks(paths, batch_size):
    # yield successive batch size path chunks from paths.
    for i in range(0, len(paths), batch_size):
        yield paths[i:i + batch_size]
        
if __name__ == "__main__":
    # parse argument
    parser = argparse.ArgumentParser("classify dataset")
    parser.add_argument('-d', '--dataset', help='dataset dir path', required=True)
    parser.add_argument('-n', '--job', help='name of the resnet job', required=True)
    parser.add_argument('-p', '--procs', default=32, help='number of parallel processes', required=False)
    parser.add_argument('-lf', 
                        '--log', 
                        default="./blobfuse2-perf.json",
                        help='path of log file', 
                        required=False)
    
    args = vars(parser.parse_args())
    
    # Preload the ResNet50 model
    resnet50.ResNet50(weights='imagenet')
    
    # create a pool of 32 threads
    dataset_path = args['dataset']
    log_file_path = args['log']
    job_name = args['job']
    procs = int(args['procs'])
    p = Pool(processes=procs)
    tic = time.time()

    sys.stdout.write('collecting images at time: {}\n'.format(time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(tic))))
    # get list of files and split them in batches of 10k to be classified
    images = [os.path.join(dp, f) for dp, dn, filenames in os.walk(dataset_path) for f in filenames]
    image_subsets = list(chunks(images, 10000))

    # load each batch onto a thread
    result = p.map(classify_images, image_subsets)
    p.close()
    p.join()
    
    toc=time.time()
    sys.stdout.write('ended processing dataset at time {}\n'.format(time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(toc))))
    sys.stdout.write('time elapsed {}\n'.format((toc-tic)))
    
    result = {job_name:{}}
    result[job_name]['time elapsed'] = toc-tic
    result[job_name]['total images'] = len(images)
    result[job_name]['images/second'] = len(images)/(toc-tic)
    
    if os.path.exists(log_file_path):
        f = open(log_file_path, mode='r+')
        data = json.load(f)
        data.update(result)
        f.seek(0)
        json.dump(data, f)
    else:
        f = open(log_file_path, mode='a+')
        json.dump(result, f)
    f.close()
