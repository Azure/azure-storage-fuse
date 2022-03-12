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
    parser.add_argument('-n', '--job-name', help='name of the resnet job', required=True)
    parser.add_argument('-lf', 
                        '--log-file', 
                        default="./blobfuse2-perf.log",
                        help='path of log file', 
                        required=False)
    
    args = vars(parser.parse_args())
    
    # create a pool of 32 threads
    p = Pool(processes=32)
    dataset_path = args['dataset']
    log_file_path = args['log-file']
    job_name = args['job-name']
    tic = time.time()

    # get list of files and split them in batches of 10k to be classified
    images = [os.path.join(dp, f) for dp, dn, filenames in os.walk(dataset_path) for f in filenames]
    image_subsets = list(chunks(images, 10000))

    # load each batch onto a thread
    result = p.map(classify_images, image_subsets)
    p.close()
    p.join()
    
    toc=time.time()
    result = {job_name:{}}
    result[job_name]['time elapsed'] = toc-tic
    result[job_name]['total images'] = len(images)
    result[job_name]['images/second'] = len(images)/(toc-tic)
    
    with open(log_file_path, 'a+') as f:
        json.dump(result, f, ensure_ascii=False)