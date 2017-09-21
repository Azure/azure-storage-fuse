#!/usr/bin/python
from subprocess import call
import os
import shutil
import time
import unittest
import random
import uuid

class TestFuse(unittest.TestCase):
    blobdir = "../build/mnt"
    localdir = "../build/temp/test"
    src = ""
    dest = ""
    blobstage = ""

    def setUp(self):
        if not os.path.exists(self.localdir):
            os.makedirs(self.localdir);
        self.src = os.path.join(self.localdir, "src");
        if not os.path.exists(self.src):
            os.makedirs(self.src);
        self.dst = os.path.join(self.localdir, "dst");
        if not os.path.exists(self.dst):
            os.makedirs(self.dst);

        self.blobstage = os.path.join(self.blobdir, "testing");
        if not os.path.exists(self.blobstage):
            os.makedirs(self.blobstage);

    def tearDown(self):
        if os.path.exists(self.blobstage):
            shutil.rmtree(self.blobstage);
        shutil.rmtree(self.localdir);

    def test_WriteReadSingleFile(self):
        print("WriteReadSingleFile")
        file1txt = "Some file1 text here."
        filepath = os.path.join(self.blobstage, "file1");
        with open(filepath, 'w') as file1blob:
            file1blob.write(file1txt)
        self.assertEqual(True, os.path.exists(filepath))
        with open(filepath, 'r') as file1blob:
            file1txtrt = file1blob.read()
            self.assertEqual(file1txt, file1txtrt)
        os.remove(filepath)
        self.assertEqual(False, os.path.exists(filepath))

    def test_medium_files(self):
        mediumBlobsSourceDir = os.path.join(self.blobstage, "mediumblobs")
        if not os.path.exists(mediumBlobsSourceDir):
            os.makedirs(mediumBlobsSourceDir);
        for i in range(0,10):
            filename = str(uuid.uuid4())
            filepath = os.path.join(mediumBlobsSourceDir, filename)
            os.system("head -c 1M < /dev/urandom > " + filepath);
            os.system("head -c 200M < /dev/zero >> " + filepath);
            os.system("head -c 1M < /dev/urandom >> " + filepath);
        files = os.listdir(mediumBlobsSourceDir)
        self.assertEqual(10, len(files))

        localBlobDir = os.path.join(self.localdir, "mediumblobs")
        if not os.path.exists(localBlobDir):
            os.makedirs(localBlobDir);
        shutil.copytree(mediumBlobsSourceDir, localBlobDir)
        files = os.listdir(localBlobDir)
        self.assertEqual(10, len(files))

        mediumBlobsDestDir = os.path.join(self.blobstage, "medium")
        if not os.path.exists(mediumBlobsDestDir):
            os.makedirs(mediumBlobsDestDir);
        shutil.copytree(localBlobDir, mediumBlobsDestDir)
        files = os.listdir(mediumBlobsDestDir)
        self.assertEqual(10, len(files))

            
    def test_directory_operations(self):
        testDir = os.path.join(self.blobstage, "testDirectory")
        self.assertFalse(os.path.exists(testDir))
        self.assertFalse(os.path.isdir(testDir))

        os.makedirs(testDir)
        self.assertTrue(os.path.exists(testDir))
        self.assertTrue(os.path.isdir(testDir))

        filetxt = "Some file text here."
        with open(os.path.join(testDir, "file1"), 'w') as fileblob:
            fileblob.write(filetxt)

        with open(os.path.join(testDir, "file2"), 'w') as fileblob:
            fileblob.write(filetxt)

        with open(os.path.join(testDir, "file3"), 'w') as fileblob:
            fileblob.write(filetxt)

        testSubDir1 = os.path.join(testDir, "subdir1")
        os.makedirs(testSubDir1)
        testSubDir2 = os.path.join(testDir, "subdir2")
        os.makedirs(testSubDir2)

        children = os.listdir(testDir);
        self.assertEqual(5, len(children))
        self.assertTrue("file1" in children)
        self.assertTrue("file2" in children)
        self.assertTrue("file3" in children)
        self.assertTrue("subdir1" in children)
        self.assertTrue("subdir2" in children)

        # Directory not empty should throw
        with self.assertRaises(OSError):
            os.rmdir(testDir)

        os.rmdir(testSubDir1)
        os.remove(os.path.join(testDir, "file2"))

        children = os.listdir(testDir)
        self.assertEqual(3, len(children))

        self.assertTrue("file1" in children)
        self.assertTrue("file3" in children)
        self.assertTrue("subdir2" in children)

        os.rmdir(testSubDir2)
        os.remove(os.path.join(testDir, "file1"))
        os.remove(os.path.join(testDir, "file3"))

        children = os.listdir(testDir)
        self.assertEqual(0, len(children))

        os.rmdir(testDir)
        self.assertFalse(os.path.exists(testDir))
        self.assertFalse(os.path.isdir(testDir))

    def test_file_operations(self):
        testFilePath = os.path.join(self.blobstage, "testfile")

        data1000 = bytes(random.randint(0, 255) for _ in range(1000))
        data500 = bytes(random.randint(0, 255) for _ in range(500))

        with open(testFilePath, 'wb') as testFile:
            testFile.write(data1000)

        self.assertEqual(1000, os.stat(testFilePath).st_size)

        with open(testFilePath, 'rb') as testFile:
            contents = testFile.read()
            self.assertEqual(data1000, contents)

        with open(testFilePath, 'ab') as testFile:
            testFile.write(data500)

        self.assertEqual(1500, os.stat(testFilePath).st_size)

        with open(testFilePath, 'rb') as testFile:
            contents = testFile.read()
            self.assertEqual(data1000 + data500, contents)

        with open(testFilePath, 'wb') as testFile:
            testFile.write(data500)

        self.assertEqual(500, os.stat(testFilePath).st_size)

        with open(testFilePath, 'rb') as testFile:
            contents = testFile.read()
            self.assertEqual(data500, contents)

if __name__ == '__main__':
    unittest.main()



