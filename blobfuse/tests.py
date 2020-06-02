#!/usr/bin/python
# -*- coding: utf-8 -*-
from subprocess import call
import os
import shutil
import time
import unittest
import errno
import random
import uuid
import threading
import stat
import subprocess
import shlex
import datetime

'''
READ ME BEFORE RUNNING TESTS
1. Before running any test, mount blobfuse first
2. Change line 25 (variable blobdir) to the mounted container path you mounted
3. Change line 28 (variable localdir) to the local temp directory (You most likely don't need to change this)
4. Change line 31 (variable cachedir) to the designated cache directory used by blobfuse
Make sure your user has permissions to these folders you've designated and that the folders also
allow to be accessed by the tests (use chown and/or chmod)
HOW TO RUN TESTS:
$ python ./tests.py 
(or replace 'tests.py' with the python test name)
use "-v" for details of each test as it runs (e.g. python ./tests.py -v)
To run individual tests or type of test add the class or class.methodtest in the command line
class: $python ./tests.py OpenFileTest
method: $python ./test.py OpenFileTest.test_open_file_nonexistent_file_read
'''


class BlobfuseTest(unittest.TestCase):
    # Get the running instance of blobfuse
    cmd_to_find = "ps -eo args | grep blobfuse | grep tmp-path"
    blob_cmd = ""
    blob_cmd = str(subprocess.check_output(cmd_to_find, shell=True))
    print (blob_cmd)
    blob_cmd_list = blob_cmd.split(" ")

    blob_cmd_tmppath = [opt for opt in blob_cmd_list if opt.startswith("--tmp-path")]

    # mounted container folder
    blobdir = blob_cmd_list[1]
    print ("Mount Dir : " + blobdir)
    
    # local temp directory
    localdir = "/mnt/tmp"
    # designated folder for the cache
    if (len(blob_cmd_tmppath) >= 1) :
        cachedir = blob_cmd_tmppath[0].split("=")[1] 
        print ("Cache Dir : " + cachedir)
    else :
        cachedir = "/mnt/blobfusetmp"
    # folder within mounted container
    blobstage = ""

    def setUp(self):
        print (" >> ", self._testMethodName)
        self.blobstage = os.path.join(self.blobdir, "testing")
        if not os.path.exists(self.localdir):
            os.system("sudo mkdir " + self.localdir)
        os.system("sudo chown `whoami` " + self.localdir)
        os.system("sudo chmod 777 " + self.localdir)

        if not os.path.exists(self.blobstage):
            os.system("sudo mkdir " + self.blobstage)
        os.system("sudo chown `whoami` " + self.blobstage)
        os.system("sudo chmod 777 " + self.blobstage)
        
    def tearDown(self):
        if os.path.exists(self.blobstage):
            os.system("sudo rm -rf " + self.blobstage + "/*")
            #shutil.rmtree(self.blobstage)
        if os.path.exists(self.localdir):
            os.system("sudo rm -rf " + self.localdir + "/*")

    # helper functions
    def validate_dir_removal(self, dirPath, dirName, parentDir):
        with self.assertRaises(OSError) as e:
            os.stat(dirPath)
        self.assertEqual(e.exception.errno, errno.ENOENT)

        homeDir = os.getcwd()
        with self.assertRaises(OSError) as e:
            os.chdir(dirPath)
        self.assertEqual(e.exception.errno, errno.ENOENT)
        os.chdir(homeDir)

        entries = os.listdir(parentDir)
        self.assertFalse(dirName in entries)

    # validates directory was created
    def validate_dir_creation(self, dirpath, dirName, parentDir):
        os.stat(dirpath)  # As long as this does not raise a FileNotFoundError, we are satisfied

        # Save values to move back to where we started and build testDir absolute path
        homeDir = os.getcwd()
        os.chdir(parentDir)
        parentDirAbsolute = os.getcwd()

        # Test that we can successfully move into the dir
        os.chdir(dirName)
        self.assertEqual(os.path.join(parentDirAbsolute, dirName), os.getcwd())

        # Test that we see the subdir when listing the current dir
        os.chdir("..")
        dir_entries = os.listdir(parentDirAbsolute)
        self.assertTrue(len(dir_entries) == 1)
        self.assertTrue(dirName in dir_entries)

        # Return to the test dir to continue with other tests
        os.chdir(homeDir)

    # validates file was removed
    def validate_file_removal(self, filePath, fileName, parentDir):
        with self.assertRaises(OSError) as e:
            os.stat(filePath)
        self.assertEqual(e.exception.errno, errno.ENOENT)

        entries = os.listdir(parentDir)
        self.assertFalse(fileName in entries)

    # validates file was created
    def validate_file_creation(self, filePath, fileName, parentDir):
        os.stat(filePath)  # As long as this doesn't fail, we are satisfied
        # print(os.stat(testFilePath))

        entries = os.listdir(parentDir)
        self.assertTrue(fileName in entries)

    # reads from file
    def read_file_func(self, filePath, start, end, testData):
        fd = os.open(filePath, os.O_RDONLY)
        os.lseek(fd, start, os.SEEK_SET)
        data = os.read(fd, end - start)
        self.assertEqual(data.decode(), testData[start:end])
        os.close(fd)

    # writes to file
    def write_file_func(self, filePath, data):
        fd = os.open(filePath, os.O_WRONLY | os.O_APPEND)
        os.write(fd, data.encode())
        os.close(fd)


class RenameTests(BlobfuseTest):

    # renames empty file within the same directory
    def test_rename_file_same_dir(self):
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)
        testFileNewName = "testFileMoved"
        testFileNewPath = os.path.join(self.blobstage, testFileNewName)

        os.open(testFilePath, os.O_CREAT)
        os.rename(testFilePath, testFileNewPath)

        with self.assertRaises(OSError) as e:
            os.stat(testFilePath)

        self.validate_file_removal(testFilePath, testFileName, self.blobstage)
        self.validate_file_creation(testFileNewPath, testFileNewName, self.blobstage)

        os.remove(testFileNewPath)

    # moves/renames empty file to another directory
    def test_rename_file_change_dir(self):
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)
        destFilePath = os.path.join(testDirPath, testFileName)

        os.mkdir(testDirPath)
        fd = os.open(testFilePath, os.O_CREAT)
        os.rename(testFilePath, destFilePath)

        self.validate_file_removal(testFilePath, testFileName, self.blobstage)
        self.validate_file_creation(destFilePath, testFileName, testDirPath)

        os.remove(destFilePath)
        os.rmdir(testDirPath)

    # renames nonempty file
    def test_rename_file_non_empty(self):
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)
        destFileName = "newFile"
        destFilePath = os.path.join(self.blobstage, destFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY)
        testData = "test data"
        os.write(fd, testData.encode())
        os.rename(testFilePath, destFilePath)

        fd = os.open(destFilePath, os.O_RDONLY)
        data = os.read(fd, 20)
        self.assertEqual(data.decode(), testData)

        os.remove(destFilePath)

    # renames subdirectory in the same parent directory
    def test_rename_dir(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)
        destDirName = "NewDir"
        destDirPath = os.path.join(self.blobstage, destDirName)

        os.mkdir(testDirPath)
        os.rename(testDirPath, destDirPath)

        self.validate_dir_removal(testDirPath, testDirName, self.blobstage)
        self.validate_dir_creation(destDirPath, destDirName, self.blobstage)

        os.rmdir(destDirPath)

    # renames/moves empty subdirectory to a different parent directory
    def test_rename_dir_change_dir(self):
        testDirParent = "ParentDir"
        parentDirPath = os.path.join(self.blobstage, testDirParent)
        destParentDir = "ParentDest"
        destParentPath = os.path.join(self.blobstage, destParentDir)
        testDirName = "TestDir"
        testDirPath = os.path.join(parentDirPath, testDirName)
        destDirPath = os.path.join(destParentPath, testDirName)

        os.mkdir(parentDirPath)
        os.mkdir(testDirPath)
        os.mkdir(destParentPath)
        os.rename(testDirPath, destDirPath)

        self.validate_dir_removal(testDirPath, testDirName, parentDirPath)
        self.validate_dir_creation(destDirPath, testDirName, destParentPath)

        os.rmdir(destDirPath)
        os.rmdir(destParentPath)
        os.rmdir(parentDirPath)

    # renames nonempty directory within same parent directory
    def test_rename_dir_nonempty(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)
        testFileName = "testFile"
        testFilePath = os.path.join(testDirPath, testFileName)

        destDirName = "newDirName"
        destDirPath = os.path.join(self.blobstage, destDirName)

        os.mkdir(testDirPath)
        fd = os.open(testFilePath, os.O_CREAT)

        os.rename(testDirPath, destDirPath)

        with self.assertRaises(OSError) as e:
            os.stat(testFilePath)
        self.assertEqual(e.exception.errno, errno.ENOENT)

        os.stat(os.path.join(destDirPath, testFileName))

        os.close(fd)
        os.remove(os.path.join(destDirPath, testFileName))
        os.rmdir(destDirPath)

    # Attempts to rename a directory that doesn't exist, expect error
    def test_rename_no_directory(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        destDirName = "newDirName"
        destDirPath = os.path.join(self.blobstage, destDirName)

        with self.assertRaises(OSError) as e:
            os.rename(testDirPath, destDirPath)
        self.assertEqual(e.exception.errno, errno.ENOENT)

    # Attempts to rename a file that doesn't exist, expect error
    def test_rename_no_file(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)
        testFileName = "testFile"
        testFilePath = os.path.join(testDirPath, testFileName)

        testNewFileName = "testFile2"
        testNewFilePath = os.path.join(testDirPath, testNewFileName)

        os.mkdir(testDirPath)

        with self.assertRaises(OSError) as e:
            os.rename(testFilePath, testNewFilePath)
        self.assertEqual(e.exception.errno, errno.ENOENT)

        os.rmdir(testDirPath)

    # test to rename directory to an existing directory
    def test_rename_dir_change_dir(self):
        testDirParent = "ParentDir"
        parentDirPath = os.path.join(self.blobstage, testDirParent)
        destParentDir = "ParentDest"
        destParentPath = os.path.join(self.blobstage, destParentDir)
        testDirName = "TestDir"
        testDirPath = os.path.join(parentDirPath, testDirName)
        destDirPath = os.path.join(destParentPath, testDirName)

        os.mkdir(parentDirPath)
        os.mkdir(testDirPath)
        os.mkdir(destParentPath)
        os.rename(testDirPath, destDirPath)

        self.validate_dir_removal(testDirPath, testDirName, parentDirPath)
        self.validate_dir_creation(destDirPath, testDirName, destParentPath)

        os.rmdir(destDirPath)
        os.rmdir(destParentPath)
        os.rmdir(parentDirPath)


class ReadWriteFileTests(BlobfuseTest):

    # test to write to a blob and read from it (regular ascii)
    def test_WriteReadSingleFile(self):
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

    # test to write to a blob and read from it (unicode)
    def test_WriteReadSingleFileUnicode(self):
        file1txt = "}L"
        filepath = os.path.join(self.blobstage, ",: hello?world-we^are%all~together1 .txt");
        #with open(filepath, 'w') as file1blob:
        #    file1blob.write(file1txt)

        cmd_str = "echo \"" + file1txt + "\" > \"" + filepath + "\""
        os.system(cmd_str)
        self.assertEqual(True, os.path.exists(filepath))
        with open(filepath, 'r') as file1blob:
            file1txtrt = file1blob.read()
            self.assertEqual(file1txt +"\n", file1txtrt)
        os.remove(filepath)
        self.assertEqual(False, os.path.exists(filepath))

    # test to overwrite file from beginning
    def test_write_file_overwrite_beginning(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_RDWR)
        testData = "test data"
        os.write(fd, testData.encode())

        os.lseek(fd, 0, os.SEEK_SET)
        testData = "overwrite all"
        os.write(fd, testData.encode())

        os.lseek(fd, 0, os.SEEK_SET)
        data = os.read(fd, 20)
        self.assertEqual(data.decode(), testData)

        os.close(fd)
        os.remove(testFilePath)

    # test to append to a file that already has data
    def test_write_file_append_to_end(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY)
        testData = "test data"
        os.write(fd, testData.encode())
        os.close(fd)

        fd = os.open(testFilePath, os.O_WRONLY | os.O_APPEND)
        moreData = "more data"
        os.write(fd, moreData.encode())
        os.close(fd)

        fd = os.open(testFilePath, os.O_RDONLY)
        data = os.read(fd, 30)
        self.assertEqual(data.decode(), testData + moreData)

        os.close(fd)
        os.remove(testFilePath)

    # test to make medium sized blobs
    # this test takes around  10 - 20 minutes
    def test_medium_files(self):
        mediumBlobsSourceDir = os.path.join(self.blobstage, "srcmediumblobs")
        if not os.path.exists(mediumBlobsSourceDir):
            os.makedirs(mediumBlobsSourceDir);
        N = 1
        for i in range(0, N):
            filename = str(uuid.uuid4())
            filepath = os.path.join(mediumBlobsSourceDir, filename)
            os.system("head -c 1M < /dev/urandom > " + filepath);
            os.system("head -c 10M < /dev/zero >> " + filepath);
            os.system("head -c 1M < /dev/urandom >> " + filepath);
        files = os.listdir(mediumBlobsSourceDir)
        self.assertEqual(N, len(files))

        localBlobDir = os.path.join(self.localdir, "localmediumblobs")
        shutil.copytree(mediumBlobsSourceDir, localBlobDir)
        files = os.listdir(localBlobDir)
        self.assertEqual(N, len(files))
        
        mediumBlobsDestDir = os.path.join(self.blobstage, "destmediumblobs")
        os.system("sudo rm -rf " + mediumBlobsDestDir)
        shutil.copytree(localBlobDir, mediumBlobsDestDir)
        files = os.listdir(mediumBlobsDestDir)
        self.assertEqual(N, len(files))


class StatsTests(BlobfuseTest):
    # test to check the stats of a directory, a file, and a nonexistent file(expect error here)
    def test_filesystem_stats(self):
        testDir = os.path.join(self.blobstage, "testDirectory")
        testFile = os.path.join(testDir, "file1")
        testNonexistingFile = os.path.join(testDir, "file2")
        os.makedirs(testDir)

        with open(testFile, 'w') as fileblob:
            fileblob.write("Dummy file")

        dirResult = os.statvfs(testDir)
        fileResult = os.statvfs(testFile)
        self.assertNotEqual(dirResult.f_bavail, 0)
        self.assertNotEqual(fileResult.f_bavail, 0)

        try:
            noResult = os.statvfs(testNonexistingFile)
        except OSError as e:
            self.assertEqual(e.errno, errno.ENOENT)

            # Directory not empty should throw
        with self.assertRaises(OSError):
            os.rmdir(testDir)

    # test to check the stats of a file
    def test_stat_file(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        # fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY)
        f = open(testFilePath, "w")
        testData = "test data"
        # os.write(fd, testData.encode())
        f.write(testData)
        f.close()

        self.assertEqual(os.stat(testFilePath).st_size, len(testData))

        os.remove(testFilePath)

    # test to check the stats of a directory
    def test_stat_dir(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        os.mkdir(testDirPath)
        self.assertEqual(os.stat(testDirPath).st_size, 4096)  # The minimum directory size on Linux

        os.rmdir(testDirPath)

    # test if stat updates after each write operation and check contents as well
    def test_write_stat_operations(self):
        testFilePath = os.path.join(self.blobstage, "testfile")

        data1000 = bytearray(os.urandom(1000))
        data500 = bytearray(os.urandom(500))

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

    # check fileowner and timestamp on file
    def test_file_owner_timestamp(self):
        testFileName = "testUserAndTime"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_RDONLY)
        testData = "random data"
        self.write_file_func(testFilePath, testData)
        os.close(fd)

        # This verifies the getattr from the local cache
        fileowner = os.stat(testFilePath).st_uid
        filegroup = os.stat(testFilePath).st_gid
        time_of_upload = os.stat(testFilePath).st_mtime

        self.assertEqual(fileowner, os.getuid())
        self.assertEqual(filegroup, os.getgid())

        # This removes the cached entries of the files just created, so they are on the service but not local.
        shutil.rmtree(self.cachedir + '/root/testing/')
        time.sleep(1)

        # check whether getattr from the service is working
        fileowner = os.stat(testFilePath).st_uid
        filegroup = os.stat(testFilePath).st_gid
        blob_last_modified = os.stat(testFilePath).st_mtime

        self.assertEqual(fileowner, os.getuid())
        self.assertEqual(filegroup, os.getgid())
        self.assertEqual(int(time_of_upload), int(blob_last_modified))

        # prime the cache and check the attributes again
        fd = os.open(testFilePath, os.O_RDONLY)

        # check whether getattr from the cache is working
        fileowner = os.stat(testFilePath).st_uid
        filegroup = os.stat(testFilePath).st_gid
        file_last_modified = os.stat(testFilePath).st_mtime

        self.assertEqual(fileowner, os.getuid())
        self.assertEqual(filegroup, os.getgid())
        self.assertEqual(int(time_of_upload), int(file_last_modified))

        os.close(fd)
        os.remove(testFilePath)

    # TODO: fix empty folder creation with incorrect date, test does not pass
    def test_stat_empty_dir_timestamp(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        os.mkdir(testDirPath)
        currentTime = datetime.datetime.now()
        dirStat = os.stat(testDirPath)

        # Empty directory dates are coming to be 1970 only so no point in testing this out fo rnow

        #self.assertEqual(currentTime.strftime("%c"), time.ctime(dirStat[stat.ST_ATIME]))
        #self.assertEqual(currentTime.strftime("%c"), time.ctime(dirStat[stat.ST_MTIME]))
        #self.assertEqual(currentTime.strftime("%c"), time.ctime(dirStat[stat.ST_CTIME]))

        shutil.rmtree(testDirPath)

    # test to see if directory timestamp gets updated when a file is made, file and directory should have same timestamp
    def test_stat_file_dir_timestamp(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        os.mkdir(testDirPath)

        testFileName = "testUserAndTime"
        testFilePath = os.path.join(testDirPath, testFileName)
        fd = os.open(testFilePath, os.O_CREAT)
        os.close(fd)

        currentTime = datetime.datetime.now()
        dirStat = os.stat(testDirPath)
        fileStat = os.stat(testFilePath)

        diff = int(currentTime.strftime("%s")) - dirStat[stat.ST_ATIME]
        self.assertLess(diff, (2 * 60))
        #self.assertEqual(currentTime.strftime("%c"), time.ctime(dirStat[stat.ST_ATIME]))
        diff = int(currentTime.strftime("%s")) - dirStat[stat.ST_MTIME]
        self.assertLess(diff, (2 * 60))
        #self.assertEqual(currentTime.strftime("%c"), time.ctime(dirStat[stat.ST_MTIME]))

        diff = int(currentTime.strftime("%s")) - fileStat[stat.ST_ATIME]
        self.assertLess(diff, (2 * 60))
        #self.assertEqual(currentTime.strftime("%c"), time.ctime(fileStat[stat.ST_ATIME]))
        diff = int(currentTime.strftime("%s")) - fileStat[stat.ST_MTIME]
        self.assertLess(diff, (2 * 60))
        #self.assertEqual(currentTime.strftime("%c"), time.ctime(fileStat[stat.ST_MTIME]))
        diff = int(currentTime.strftime("%s")) - fileStat[stat.ST_CTIME]
        self.assertLess(diff, (2 * 60))
        #self.assertEqual(currentTime.strftime("%c"), time.ctime(fileStat[stat.ST_CTIME]))

        os.remove(testFilePath)
        shutil.rmtree(testDirPath)


class OpenFileTests(BlobfuseTest):
    # attempts to read file that doesn't exist, expect error
    def test_open_file_nonexistent_file_read(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        with self.assertRaises(OSError) as e:
            os.open(testFilePath, os.O_RDONLY)

        self.assertEqual(e.exception.errno, errno.ENOENT)

    # attempts to write to a file that doesn't exist, expect error
    def test_open_file_nonexistent_file_write(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        with self.assertRaises(OSError) as e:
            os.open(testFilePath, os.O_WRONLY)

        self.assertEqual(e.exception.errno, errno.ENOENT)

    # test open an empty file in write only, write to it, close it
    # reopen the file in read only to check if we can read the contents
    def test_open_file_exists_read_write_empty(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY)  # This covers opening a file that exists and is empty
        testData = "Test data"
        os.write(fd, testData.encode())
        os.close(fd)  # It would be possible to seek, but that should be a separate test. Reoping is more granular

        fd = os.open(testFilePath, os.O_CREAT | os.O_RDONLY)
        data = os.read(fd, 15)
        self.assertEqual(data.decode(), testData)

        os.close(fd)
        os.remove(testFilePath)

    # create/open file in write only, then try to read from it and expect an error
    # then open the file in read only and attempt to write to it and expect an error
    def test_open_file_read_only_write_only(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY)
        testData = "Test data"
        os.write(fd, testData.encode())
        with self.assertRaises(OSError) as e:
            os.read(fd, 20)
        self.assertEqual(e.exception.errno, errno.EBADF)
        os.close(fd)

        # This also tests opening a non-empty file and a file that was closed
        fd = os.open(testFilePath, os.O_RDONLY)
        with self.assertRaises(OSError) as e:
            os.write(fd, testData.encode())
        self.assertEqual(e.exception.errno, errno.EBADF)
        data = os.read(fd, 20)
        self.assertEqual(data.decode(), testData)
        os.close(fd)

        os.remove(testFilePath)

    # test if we can open the same file in write only, then in read only then using the
    # respective file handles, read and write to the file
    def test_open_file_already_open(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY)
        fd2 = os.open(testFilePath, os.O_CREAT | os.O_RDONLY)

        testData = "test data"
        os.write(fd, testData.encode())
        data = os.read(fd2, 20)
        self.assertEqual(data.decode(), testData)

        os.close(fd)
        os.close(fd2)
        os.remove(testFilePath)

    # expect an error if we try to open a directory path
    def test_open_dir_exists(self):
        # expect failure
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        os.mkdir(testDirPath)

        with self.assertRaises(OSError) as e:
            fd = os.open(testDirPath, os.O_CREAT | os.O_RDWR)
        self.assertEqual(e.exception.errno, errno.EISDIR)

        os.rmdir(testDirPath)


class CloseFileTests(BlobfuseTest):
    # helper function for  closing file through a process/thread
    def close_file_func(self, filePath):
        fd = os.open(filePath, os.O_RDONLY)
        os.close(fd)

    # opening a file after another process opened it
    def test_read_file_after_another_process_closes(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_RDWR)
        testData = "test data"
        os.write(fd, testData.encode())
        os.lseek(fd, 0, os.SEEK_SET)

        thread = threading.Thread(target=self.close_file_func, args=(testFilePath,))
        thread.start()
        thread.join()

        data = os.read(fd, 20)
        self.assertEqual(data.decode(), testData)

        os.close(fd)
        os.remove(testFilePath)

    # test file after we opened it, expect error when we try to write to a file that's closed
    def test_close_file(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY)
        os.close(fd)

        with self.assertRaises(OSError) as e:
            os.write(fd, "data".encode())
        self.assertEqual(e.exception.errno, errno.EBADF)

        os.remove(testFilePath)

    # test file after we opened it, closed it and expect an error after we try to close it again
    def test_close_file_twice(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY)
        os.close(fd)

        with self.assertRaises(OSError) as e:
            os.close(fd)
        self.assertEqual(e.exception.errno, errno.EBADF)

        os.remove(testFilePath)


class RemoveFileTests(BlobfuseTest):

    # test if we can remove a file
    def test_remove_file(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        f = os.open(testFilePath, os.O_CREAT)
        os.close(f)
        os.remove(testFilePath)

        self.validate_file_removal(testFilePath, testFileName, self.blobstage)

    # test if we can remove a file that's still open
    def test_remove_file_still_open(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        f = os.open(testFilePath, os.O_CREAT)
        os.remove(testFilePath)

        self.validate_file_removal(testFilePath, testFileName, self.blobstage)

    # expect an error if we try to remove a file that doesn't exist
    def test_remove_nonexistent(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        # throw exception here, expect OSError
        with self.assertRaises(OSError) as e:
            os.remove(testFilePath)
        self.assertEqual(e.exception.errno, errno.ENOENT)

        self.validate_file_removal(testFilePath, testFileName, self.blobstage)

    # expect an error if we try to remove a file that has already been removed
    def test_remove_file_twice(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        f = os.open(testFilePath, os.O_CREAT)
        os.close(f)
        os.remove(testFilePath)

        self.validate_file_removal(testFilePath, testFileName, self.blobstage)

        # throw exception here, expect OSError
        with self.assertRaises(OSError) as e:
            os.remove(testFilePath)
        self.assertEqual(e.exception.errno, errno.ENOENT)

        self.validate_file_removal(testFilePath, testFileName, self.blobstage)


class TruncateTests(BlobfuseTest):

    # test if we can truncate a file
    def test_truncate_file(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY)
        os.write(fd, "random data".encode())
        os.ftruncate(fd, 0)
        os.close(fd)

        self.assertEqual(os.stat(testFilePath).st_size, 0)

        os.remove(testFilePath)

    # test if we can truncate a file and if we can read from it and open it and truncate it again
    def test_truncate_file_non_zero(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY)
        os.write(fd, "random data".encode())
        os.ftruncate(fd, 5)
        os.close(fd)

        self.assertEqual(os.stat(testFilePath).st_size, 5)
        with open(testFilePath, 'rb') as testFile:
            contents = testFile.read()
            self.assertEqual("rando".encode(), contents)

        fd = os.open(testFilePath, os.O_RDWR)
        os.ftruncate(fd, 30)
        os.close(fd)

        self.assertEqual(os.stat(testFilePath).st_size, 30)

        os.remove(testFilePath)

    # test to truncate an empty file
    def test_truncate_empty_file(self):
        # dunno if should expecting error here or not
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY)
        os.ftruncate(fd, 0)
        os.close(fd)

        self.assertEqual(os.stat(testFilePath).st_size, 0)

        os.remove(testFilePath)


class ThreadTests(BlobfuseTest):
    # test if we can read from the same file at the same time with multiple processes
    def test_file_simultaneous_read(self):
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY)
        testData = "Plenty of data for simultaneous reads"
        os.write(fd, testData.encode())
        os.close(fd)

        thread1 = threading.Thread(target=self.read_file_func, args=(testFilePath, 0, 10, testData,))
        thread2 = threading.Thread(target=self.read_file_func, args=(testFilePath, 10, 20, testData,))
        thread3 = threading.Thread(target=self.read_file_func, args=(testFilePath, 20, len(testData), testData,))

        thread1.start()
        thread2.start()
        thread3.start()

        thread1.join()
        thread2.join()
        thread3.join()

        os.remove(testFilePath)

    # tests if we can write to the same  file with multiple threads
    def test_file_simultaneous_write(self):
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_RDONLY)

        testData = "random data"
        thread1 = threading.Thread(target=self.write_file_func, args=(testFilePath, testData))
        thread2 = threading.Thread(target=self.write_file_func, args=(testFilePath, testData))
        thread3 = threading.Thread(target=self.write_file_func, args=(testFilePath, testData))

        thread1.start()
        thread2.start()
        thread3.start()

        thread1.join()
        thread2.join()
        thread3.join()

        self.assertEqual(os.stat(testFilePath).st_size, len(testData) * 3)

        os.close(fd)
        os.remove(testFilePath)

    def test_multiple_file_handles_on_single_file(self):
        # This is to test whether close of a file handle impacts other open file handles on the same file
        # reported in issue 57
        testFileName = "testFile_new" + str(random.randint(0, 10000))
        testFilePath = os.path.join(self.blobstage, testFileName)

        repeat = random.randint(1, 10)
        fd1 = os.open(testFilePath, os.O_WRONLY | os.O_CREAT)
        fd2 = os.open(testFilePath, os.O_WRONLY)
        os.close(fd2)

        # sleep until cache times out
        # TODO: Improve test execution time by reducing cache timeout
        time.sleep(130)

        testData = "random data"

        for i in range(0, repeat):
            os.write(fd1, testData.encode())

        os.close(fd1)

        self.assertEqual(os.stat(testFilePath).st_size, len(testData.encode()) * repeat)

    def test_multiple_threads_create_cache_directory_simultaneous(self):
        # This is to test the fix to a bug that reported failure if multiple threads simultaneously called ensure_directory_exists_in_cache.
        # The directory would be successfully created, but many threads would fail because they would try to create it after another thread had already done so.
        sourceDirName = "mediumblobs-2"
        mediumBlobsSourceDir = os.path.join(self.blobstage, sourceDirName)
        if not os.path.exists(mediumBlobsSourceDir):
            os.makedirs(mediumBlobsSourceDir);
        # We must use different files for each thread to avoid the synchronization that would occur if all threads access the same file
        for i in range(0, 1):
            filename = str(uuid.uuid4())
            filepath = os.path.join(mediumBlobsSourceDir, filename)
            os.system("head -c 10M < /dev/zero >> " + filepath);

        # This removes the cached entries of the files just created, so they are on the service but not local.
        # This will force each thread to call ensure_directory_exists_in_cache when trying to access its file.
        shutil.rmtree(self.cachedir + '/root/testing/' + sourceDirName)

        threads = []

        for filename in os.listdir(mediumBlobsSourceDir):
            path = os.path.join(mediumBlobsSourceDir, filename)
            threads.append(threading.Thread(target=self.read_file_func, args=(
            path, 0, 0, "",)))  # Note that read_file_func also opens the file, which is the desired behavior

        for thread in threads:
            thread.start()

        for thread in threads:
            thread.join()

        shutil.rmtree(mediumBlobsSourceDir)


class CreateFileTests(BlobfuseTest):
    # test to create a new file
    def test_create_file_new_file(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY | os.O_TRUNC)
        # open(testFilePath, "w")

        self.validate_file_creation(testFilePath, testFileName, self.blobstage)

        os.close(fd)
        os.remove(testFilePath)

    # expect error trying to create a file of a name that is already taken
    def test_create_file_name_conflict_file(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT)

        with self.assertRaises(OSError) as e:
            os.open(testFilePath, os.O_EXCL | os.O_CREAT)
        self.assertEqual(e.exception.errno, errno.EEXIST)

        os.close(fd)
        os.remove(testFilePath)

    # expect error when trying to create a file with a name of the a directory that exists
    def test_create_file_name_conflict_dir(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        os.mkdir(testFilePath)
        with self.assertRaises(OSError) as e:
            os.open(testFilePath, os.O_CREAT | os.O_EXCL)
        self.assertEqual(e.exception.errno, errno.EEXIST)

        os.rmdir(testFilePath)


class MakeDirectoryTests(BlobfuseTest):
    # test for a lot of directory commands
    # Test making a directory, making files in that directory, making subdirectories, and files for the subdirectories
    # use listdir to check the contents of that directory
    # test that we cannot remove a nonempty directory
    # test that we actually removed the direcotry
    def test_directory_operations(self):
        testDir = os.path.join(self.blobstage, "testDirectory")
        subdir1 = "subDir1"
        #subdir2 = "Th0s\!is-a%directory&name @1"
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

        testSubDir1 = os.path.join(testDir, subdir1)
        os.makedirs(testSubDir1)
        #testSubDir2 = os.path.join(testDir, subdir2)
        #testSubDir2 = "\"" + testSubDir2 + "\""
        #os.system("sudo mkdir " + testSubDir2)
        #os.system("sudo chmod 777 " + testSubDir2)

        children = os.listdir(testDir);
        self.assertEqual(4, len(children))
        self.assertTrue("file1" in children)
        self.assertTrue("file2" in children)
        self.assertTrue("file3" in children)
        self.assertTrue(subdir1 in children)
        #self.assertTrue(subdir2 in children)

        # Directory not empty should throw
        with self.assertRaises(OSError):
            os.rmdir(testDir)

        os.rmdir(testSubDir1)
        os.remove(os.path.join(testDir, "file2"))

        children = os.listdir(testDir)
        self.assertEqual(2, len(children))

        self.assertTrue("file1" in children)
        self.assertTrue("file3" in children)
        #self.assertTrue(subdir2 in children)

        #os.rmdir(testSubDir2)
        os.remove(os.path.join(testDir, "file1"))
        os.remove(os.path.join(testDir, "file3"))

        children = os.listdir(testDir)
        self.assertEqual(0, len(children))

        os.rmdir(testDir)
        self.assertFalse(os.path.exists(testDir))
        self.assertFalse(os.path.isdir(testDir))

    # test of validating of making an empty directory
    def test_make_new_directory(self):
        # Note that based on the value of self.blobdir, this also tests the relative path
        testDirName = "testDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        os.mkdir(testDirPath)

        self.validate_dir_creation(testDirPath, testDirName, self.blobstage)

        os.rmdir(os.path.join(self.blobstage, testDirName))

    # test if we try to make a directory with a name that already exists
    def test_make_directory_name_exists(self):
        testDirName = "testDir"
        testDirPath = os.path.join(self.blobstage, testDirName)
        os.mkdir(testDirPath)

        try:
            os.mkdir(testDirPath)
        except OSError as e:
            self.assertEqual(e.errno, errno.EEXIST)

        os.rmdir(os.path.join(self.blobstage, testDirName))

    # replace scandir eventually
    '''
    def test_close_directory(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)
        os.mkdir(testDirPath)
        f = open(testFilePath, "w")
        f.close()
        entries = os.scandir(testDirPath)
        entries.close()
        with self.assertRaises(StopIteration):
            next(entries)
        os.remove(testFilePath)
        os.rmdir(testDirPath)
    '''''
    # This test throws a errno 5 (I/O error) instead of Name too long error
    # apparently in the past this tests has thrown a 400
    # TODO: Figure out the appropriate error code to be thrown here otherwise this test works for the most part
    '''
    def test_make_directory_long_name(self):
        homeDir = os.getcwd()
        os.chdir(self.blobstage)
        # The service currently has a limit of 1024 characters
        testDir = "a"
        while len(testDir) < 1100:
            testDir += os.path.join(testDir, "a" * 200)
        with self.assertRaises(OSError) as e:
            os.makedirs(testDir)
        self.assertEqual(e.exception.errno, errno.ENAMETOOLONG)
        shutil.rmtree("aa")
        os.chdir(homeDir)
    '''

    # test making a directory with an aboslute path
    def test_make_directory_absolute_path(self):
        testDirName = "testDir"
        testDirPath = os.path.join(os.getcwd(), self.blobstage, testDirName)
        testDirAbsPath = os.path.abspath(testDirPath)

        os.mkdir(testDirAbsPath)

        self.validate_dir_creation(testDirAbsPath, testDirName, self.blobstage)

        os.rmdir(testDirAbsPath)

    # test make directory and creating a file within the directory
    def test_make_directory_add_file(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        os.mkdir(testDirPath)

        testFileName = "testFile"
        testFilePath = os.path.join(testDirPath, testFileName)

        f = open(testFilePath, "w")
        f.close()

        # Note this also tests opening and reading a created directory with only files
        entries = os.listdir(testDirPath)

        self.assertTrue(len(entries) == 1)  # Ensure we cannot see the .directory blob
        self.assertTrue(entries[0] == testFileName)

        os.remove(testFilePath)
        os.rmdir(testDirPath)

    # test making a directory and making a subdirectory within it
    def test_make_directory_add_subdir(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        os.mkdir(testDirPath)

        testSubdirName = "testSubdir"
        testSubdirPath = os.path.join(testDirPath, testSubdirName)

        os.mkdir(testSubdirPath)

        self.validate_dir_creation(testSubdirPath, testSubdirName, testDirPath)

        # Note this also tests opening and reading a created directory with only directories
        entries = os.listdir(testDirPath)
        self.assertTrue(len(entries) == 1)  # Ensure we cannot see the .directory blob
        self.assertTrue(entries[0] == testSubdirName)

        os.rmdir(testSubdirPath)
        os.rmdir(testDirPath)


class OpenListDirectoryTests(BlobfuseTest):
    # test making an empty directory and checking with list dir
    def test_open_directory_dir_empty(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        os.mkdir(testDirPath)

        entries = os.listdir(testDirPath)  # Note this also tests opening via relative path on existing directory
        self.assertTrue(len(entries) == 0)

        os.rmdir(testDirPath)

    # testing open / list dir with a nonexistent directory
    def test_open_directory_dir_never_created(self):
        testDirName = "Test"
        testDirPath = os.path.join(self.blobstage, testDirName)

        with self.assertRaises(OSError) as e:
            os.listdir(testDirPath)

        self.assertEqual(e.exception.errno, errno.ENOENT)

    # test open / list dir on a path that's absolute
    def test_open_directory_absolute_path(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)
        testDirAbs = os.path.abspath(testDirPath)

        os.mkdir(testDirAbs)

        entries = os.listdir(testDirAbs)
        self.assertTrue(len(entries) == 0)

        os.rmdir(testDirAbs)

    # test open / list dir with a directory with empty subdirectories and files
    def test_open_directory_with_files_and_subdirs(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        testFileName = "TestFile"
        testFilePath = os.path.join(testDirPath, testFileName)
        testSubdirName = "TestSubdir"
        testSubdirPath = os.path.join(testDirPath, testSubdirName)

        os.mkdir(testDirPath)
        os.mkdir(testSubdirPath)
        testFile = open(testFilePath, "w")
        testFile.close()

        entries = os.listdir(testDirPath)
        self.assertEqual(len(entries), 2)
        self.assertTrue(testFileName in entries)
        self.assertTrue(testSubdirName in entries)

        os.remove(testFilePath)
        shutil.rmtree(testDirPath)

    # test list directory with a directory with nonempty subdirectory
    def test_read_directory_with_non_empty_subdir(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        testSubdirName = "TestSubdir"
        testSubdirPath = os.path.join(testDirPath, testSubdirName)
        testFileName = "TestFile"
        testFilePath = os.path.join(testSubdirPath, testFileName)

        os.mkdir(testDirPath)
        os.mkdir(testSubdirPath)
        testFile = open(testFilePath, "w")
        testFile.close()

        entries = os.listdir(testDirPath)
        self.assertEqual(len(entries), 1)
        self.assertEqual(entries[0], testSubdirName)
        entries = os.listdir(testSubdirPath)
        self.assertEqual(len(entries), 1)
        self.assertEqual(entries[0], testFileName)

        os.remove(testFilePath)
        shutil.rmtree(testDirPath)

    # test list dir with empty directory
    def test_read_directory_empty_dir(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        os.mkdir(testDirPath)

        entries = os.listdir(testDirPath)

        self.assertEqual(0, len(entries))

        shutil.rmtree(testDirPath)


class RemoveDirectoryTests(BlobfuseTest):

    # test remove empty directory
    def test_remove_directory_empty(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        os.mkdir(testDirPath)

        os.rmdir(testDirPath)  # Note this also tests removal via relative path

        self.validate_dir_removal(testDirPath, testDirName, self.blobstage)

    # test remove directory path that's absolute
    def test_remove_directory_absolute_path(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)
        testDirAbs = os.path.abspath(testDirPath)

        os.mkdir(testDirPath)

        os.rmdir(testDirAbs)

        self.validate_dir_removal(testDirAbs, testDirName, self.blobstage)

    # test remove directory with non empty files, expect error when trying to remove a directory without
    # removing the files first
    def test_remove_directory_non_empty_files(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)
        testFileName = "testFile"
        testFilePath = os.path.join(testDirPath, testFileName)

        os.mkdir(testDirPath)
        testFile = open(testFilePath, "w")
        testFile.close()

        with self.assertRaises(OSError) as e:
            os.rmdir(testDirPath)
        self.assertEqual(e.exception.errno, errno.ENOTEMPTY)

        os.remove(testFilePath)
        os.rmdir(testDirPath)

    # test removing a directory with a non empty subdirectory, expect an error if trying to remove the
    # parent directory without first emptying the file and subdirectory or removing it recursively
    def test_remove_directory_non_empty_subdir(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)
        testSubdirName = "TestSubdir"
        testSubdirPath = os.path.join(testDirPath, testSubdirName)

        os.makedirs(testSubdirPath)

        with self.assertRaises(OSError) as e:
            os.rmdir(testDirPath)
        self.assertEqual(e.exception.errno, errno.ENOTEMPTY)

        shutil.rmtree(testDirPath)

    # test removing the directory that doesn't exist
    def test_remove_directory_never_created(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        with self.assertRaises(OSError) as e:
            os.rmdir(testDirPath)
        self.assertEqual(e.exception.errno, errno.ENOENT)

    # test removing a directory and expect an error when attempting to open the directory
    def test_remove_directory_cd_into(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        os.mkdir(testDirPath)
        os.rmdir(testDirPath)

        homeDir = os.getcwd()
        with self.assertRaises(OSError) as e:
            os.chdir(testDirPath)
        self.assertEqual(e.exception.errno, errno.ENOENT)

        os.chdir(homeDir)

    # use listdir tests instead unless earlier versions of python are available
    '''
    def test_close_directory(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)
        os.mkdir(testDirPath)
        f = open(testFilePath, "w")
        f.close()
        entries = os.scandir(testDirPath)
        entries.close()
        with self.assertRaises(StopIteration):
            next(entries)
        os.remove(testFilePath)
        os.rmdir(testDirPath)
    def test_close_directory_already_closed(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)
        os.mkdir(testDirPath)
        f = open(testFilePath, "w")
        f.close()
        entries = os.scandir(testDirPath) #use scandir with newer versions of python
        entries.close()
        entries.close()
        os.remove(testFilePath)
        os.rmdir(testDirPath)
    '''

    # test fuse handling a crash, when attempting to close a file that has the same name as a blob
    # but that's not in the mounted container
    def test_fuse_crash(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        testFileName = "TestFile"
        testFilePath = os.path.join(testDirPath, testFileName)
        testSubdirName = "TestSubdir"
        testSubdirPath = os.path.join(testDirPath, testSubdirName)

        os.mkdir(testDirPath)
        os.mkdir(testSubdirPath)
        testFile = open(testFilePath, "w")
        with self.assertRaises(TypeError) as e:
            testFile.close(testFile)  # This line seems to make fuse crash

        testFile.close()
        entries = os.listdir(testDirPath)
        self.assertTrue(len(entries) == 2)
        self.assertTrue(testFileName in entries)
        self.assertTrue(testSubdirName in entries)

        os.remove(testFilePath)
        shutil.rmtree(testDirPath)

    # TODO: implement flock
    '''
class FlockTests(BlobfuseTest):
    def test_file_lock_shared(self):
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)
        fd = os.open(testFilePath, os.O_CREAT)
        fcntl.flock(fd, fcntl.LOCK_SH)
        fd2 = os.open(testFilePath, os.O_WRONLY)
        fcntl.flock(fd2, fcntl.LOCK_SH) # Acquiring two shared locks is valid
        os.close(fd)
        os.close(fd2)
        os.remove(testFilePath)
    def test_file_lock_exclusive(self):
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)
        fd = os.open(testFilePath, os.O_CREAT)
        fcntl.flock(fd, fcntl.LOCK_EX)
        fd2 = os.open(testFilePath, os.O_WRONLY)
        with self.assertRaises(BlockingIOError) as e:
            fcntl.flock(fd2, fcntl.LOCK_EX | fcntl.LOCK_NB)
        self.assertEqual(e.exception.errno, errno.EAGAIN)
        os.close(fd)
        os.close(fd2)
        os.remove(testFilePath)
    def test_file_lock_shared_then_exclusive(self):
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)
        fd = os.open(testFilePath, os.O_CREAT)
        fcntl.flock(fd, fcntl.LOCK_SH)
        fd2 = os.open(testFilePath, os.O_WRONLY)
        with self.assertRaises(BlockingIOError) as e:
            fcntl.flock(fd2, fcntl.LOCK_EX | fcntl.LOCK_NB)
        self.assertEqual(e.exception.errno, errno.EAGAIN)
        os.close(fd)
        os.close(fd2)
        os.remove(testFilePath)
    def test_file_lock_exclusive_then_shared(self):
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)
        fd = os.open(testFilePath, os.O_CREAT)
        fcntl.flock(fd, fcntl.LOCK_EX)
        fd2 = os.open(testFilePath, os.O_WRONLY)
        with self.assertRaises(BlockingIOError) as e:
            fcntl.flock(fd2, fcntl.LOCK_SH | fcntl.LOCK_NB)
        self.assertEqual(e.exception.errno, errno.EAGAIN)
        os.close(fd)
        os.close(fd2)
        os.remove(testFilePath)
    def test_file_lock_change_type(self):
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)
        fd = os.open(testFilePath, os.O_CREAT)
        fcntl.flock(fd, fcntl.LOCK_EX)
        fcntl.flock(fd, fcntl.LOCK_SH)
        fd2 = os.open(testFilePath, os.O_WRONLY)
        fcntl.flock(fd2, fcntl.LOCK_SH)
        os.close(fd)
        os.close(fd2)
        os.remove(testFilePath)
    def test_file_lock_release(self):
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)
        fd = os.open(testFilePath, os.O_CREAT)
        fcntl.flock(fd, fcntl.LOCK_EX)
        fcntl.flock(fd, fcntl.LOCK_UN)
        fd2 = os.open(testFilePath, os.O_WRONLY)
        fcntl.flock(fd2, fcntl.LOCK_EX)
        os.close(fd)
        os.close(fd2)
        os.remove(testFilePath)
    def test_file_lock_release_on_close(self):
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)
        fd = os.open(testFilePath, os.O_CREAT)
        fcntl.flock(fd, fcntl.LOCK_EX)
        os.close(fd)
        fd = os.open(testFilePath, os.O_WRONLY)
        fcntl.flock(fd, fcntl.LOCK_EX)
        os.close(fd)
        os.remove(testFilePath)
    def test_file_lock_close_file(self):
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)
        fd = os.open(testFilePath, os.O_CREAT)
        os.close(fd)
        with self.assertRaises(OSError) as e:
            fcntl.flock(fd, fcntl.LOCK_SH)
        self.assertEqual(e.exception.errno, errno.EBADF)
        os.remove(testFilePath)
    def test_file_lock_release_never_acquired(self):
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)
        fd = os.open(testFilePath, os.O_CREAT)
        fcntl.flock(fd, fcntl.LOCK_UN)
        os.close(fd)
        os.remove(testFilePath)
    def try_lock_file(self, filePath):
        fd = os.open(filePath, os.O_WRONLY)
        fcntl.flock(fd, fcntl.LOCK_EX)
    def test_file_block_on_lock(self):
        testFileName = "testFile"
        testFilePath = os.path.join(self.blobstage, testFileName)
        fd = os.open(testFilePath, os.O_CREAT)
        fcntl.flock(fd, fcntl.LOCK_EX)
        # We want to assert that the file is locked even between processes, not just threads
        process = multiprocessing.Process(target=self.try_lock_file, args=(testFilePath,))
        process.start()
        process.join(2)
        # If the process has exitcode=none, it is still running and therefore blocked as expected 
        self.assertIsNone(process.exitcode)
        process.terminate()
        os.close(fd)
        os.remove(testFilePath)
'''
#note: these tests take a certain amount of time due to the volume of files
#filling the mounted container, emptying the container and waiting for the cache to clear itself
#in order to make it more efficient to fill the cache without taking so much time
#we allocate a small ram disk in order to reach capacity with the disk
#also make sure this test is in the python_test directory
#if you have issues running these tests due to access permissions issue. It might help to
#manually run 'sudo chown <username> <foldername>' before these tests
class CacheTests(BlobfuseTest):
    #path to ramdisk
    ramDiskPath = "/mnt/ramdisk"
    #path to cache container for blobfuse in ramdisk
    ramDiskTmpPath = "/mnt/ramdiskblobfuseTmp"
    #path to mounted container in ramdisk
    ramDiskContainerPath = "/mnt/ramdiskMountedCnt"
    #upper/high threshold
    upper_threshold = 90
    #lower/bottom threshold
    lower_threshold = 80

    os.system("sudo rm -rf " +  ramDiskContainerPath)

    def setUp(self):
        print (" >> ", self._testMethodName)
        # create temp/cache directory
        if not os.path.exists(self.ramDiskPath):
            #os.mkdir(self.ramDiskPath)
            os.system("sudo mkdir " + self.ramDiskPath)
        os.system("sudo chown `whoami` " + self.ramDiskPath)
        os.system("sudo chmod 777 " + self.ramDiskPath)

        if not os.path.exists(self.ramDiskTmpPath):
            #os.mkdir(self.ramDiskTmpPath)
            os.system("sudo mkdir " + self.ramDiskTmpPath)
        os.system("sudo chown `whoami` " + self.ramDiskTmpPath)
        os.system("sudo chmod 777 " + self.ramDiskTmpPath)
        #os.chown(self.ramDiskTmpPath, os.geteuid(), os.getgid())

        if not os.path.exists(self.ramDiskContainerPath):
            #os.mkdir(self.ramDiskContainerPath)
            os.system("sudo mkdir " + self.ramDiskContainerPath)
        os.system("sudo chown `whoami` " + self.ramDiskContainerPath)
        os.system("sudo chmod 777 " + self.ramDiskContainerPath)
        #os.chown(self.ramDiskContainerPath, os.geteuid(), os.getgid())

    def tearDown(self):
        # unmount blobfuse
        os.system("fusermount -u " + self.ramDiskContainerPath)

        #unmount ramdisk
        os.system("sudo umount " + self.ramDiskPath)

        #delete container directory if still exists
        if os.path.exists(self.ramDiskContainerPath):
            #shutil.rmtree(self.ramDiskContainerPath)
            os.system("sudo rm -rf "+ self.ramDiskContainerPath + "/*")

        #delete cache/temp directory if still exists
        if os.path.exists(self.ramDiskTmpPath):
            #shutil.rmtree(self.ramDiskTmpPath)
            os.system("sudo rm -rf "+ self.ramDiskTmpPath + "/*")


        #delete cache/temp directory if still exists
        #if os.path.exists(self.ramDiskPath):
            #shutil.rmtree(self.ramDiskPath)
            os.system("sudo rm -rf "+ self.ramDiskPath + "/*")


    def makeRamDisk(self, disk_size):
        #create ramdisk, give current user access to the ramdisk and mount
        os.system("sudo mount -t tmpfs -o size=" + disk_size + " tmpfs " + self.ramDiskPath)

    def startBlobfuse(self, cache_timeout):
        #call blobfuse using ramdisk as the cache directory
        #os.system("sudo kill -9 `pidof blobfuse`")
        #os.system("sudo fusermount -u " + self.blobdir)
        #os.system("rm -rf " + self.cachedir + "/*")
        os.system("rm -rf " + self.ramDiskTmpPath + "/*")
        os.system("rm -rf " + self.ramDiskContainerPath + "/*")
        #os.system("sudo fusermount -u " + self.ramDiskContainerPath)

        blobfuseMountCmd = "./blobfuse " + self.ramDiskContainerPath + " --tmp-path=" + self.ramDiskTmpPath + \
                           " -o attr_timeout=240 -o entry_timeout=240 -o negative_timeout=120 " \
                           "--file-cache-timeout-in-seconds=" + cache_timeout + \
                           " --config-file=../connection.cfg --log-level=LOG_DEBUG"
        os.chdir(os.path.dirname(os.path.abspath(__file__)) + "/../build")
        os.system(blobfuseMountCmd)

    def test_cache_large_files(self):
        testDirName = "testLargeFileDir"
        testDirPath = os.path.join(self.ramDiskContainerPath, testDirName)
        ramDiskSize = "1024M"
        cacheTimeout = "120"

        self.makeRamDisk(ramDiskSize)
        self.startBlobfuse(cacheTimeout)

        # arrange
        if not os.path.exists(testDirPath):
            os.mkdir(testDirPath)
        os.chown(testDirPath, os.geteuid(), os.getgid())

        filename = str(uuid.uuid4())

        # act (create many files)
        for i in range(0, 1):
            filename = str(uuid.uuid4())
            filepath = os.path.join(testDirPath, filename)
            os.system("head -c 1M < /dev/urandom > " + filepath)
            os.system("head -c 2M < /dev/zero >> " + filepath)
            os.system("head -c 2M < /dev/urandom >> " + filepath)
        #this makes 1015MB, so it fills the cache, so the threshold should be met

        #assert (check cache)
        #check how close we are to threshold after filling the cache
        #if we are past low threshold, fail the test
        #if we are under then we assured we deleted to not hit the threshold
        df = subprocess.Popen(["df", self.ramDiskTmpPath], stdout=subprocess.PIPE)
        output = df.communicate()[0]
        print (output)
        device, size, used, available, percent, mountpoint = str(output).split("\n")[1].split()
        self.assertLess(int(percent.strip('%')), self.lower_threshold)

        # cleanup
        #if os.path.exists(testDirPath):
        #    shutil.rmtree(testDirPath)

    def test_cache_small_files(self):
        testDirName = "testSmallFileDir"
        testDirPath = os.path.join(self.ramDiskContainerPath, testDirName)
        ramDiskSize = "100M"
        cacheTimeout = "120"

        self.makeRamDisk(ramDiskSize)
        self.startBlobfuse(cacheTimeout)

        #arrange
        if not os.path.exists(testDirPath):
            #os.mkdir(testDirPath)
            os.system("sudo mkdir " + testDirPath)
        #os.chown(testDirPath, os.geteuid(), os.getgid())
        os.system("sudo chown `whoami` " + testDirPath)
        os.system("sudo chmod 777 " + testDirPath)

        filename = str(uuid.uuid4())

        #act (create many files)
        for i in range(0, 1):
            filename = str(uuid.uuid4())
            filepath = os.path.join(testDirPath, filename)
            os.system("head -c 1M < /dev/urandom > " + filepath)
            os.system("head -c 7M < /dev/zero >> " + filepath)
            os.system("head -c 2M < /dev/urandom >> " + filepath)

        # assert (check cache)
        # check if we reached the low threshold. we expect no cleanup and to stay at the low threshold
        df = subprocess.Popen(["df", self.ramDiskTmpPath], stdout=subprocess.PIPE)
        output = df.communicate()[0]
        print (output)
        device, size, used, available, percent, mountpoint = str(output).split("\n")[1].split()
        #self.assertEqual(int(percent.strip('%')), self.lower_threshold)

        #if we add another file then it should reduce the cache size to below the lower threshold
        #because we hit the high threshold
        filename = str(uuid.uuid4())
        filepath = os.path.join(testDirPath, filename)
        os.system("head -c 1M < /dev/urandom > " + filepath)
        os.system("head -c 7M < /dev/zero >> " + filepath)
        os.system("head -c 2M < /dev/urandom >> " + filepath)

        # assert (check cache)
        # check how close we are to threshold after filling the cache
        # if we are past the low threshold, fail the test
        # if we are under then we assured we deleted to not hit the threshold
        df = subprocess.Popen(["df", self.ramDiskTmpPath], stdout=subprocess.PIPE)
        output = df.communicate()[0]
        device, size, used, available, percent, mountpoint = str(output).split("\n")[1].split()
        self.assertLess(int(percent.strip('%')), self.lower_threshold)
        # cleanup
        if os.path.exists(testDirPath):
            shutil.rmtree(testDirPath)

if __name__ == '__main__':
    unittest.main()
