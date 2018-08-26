#!/usr/bin/python
# This Python file uses the following encoding: utf-8
from subprocess import call
import os
import shutil
import time
import unittest
import random
import uuid
import ctypes
import errno
import stat
import threading
import fcntl
import multiprocessing

class TestFuse(unittest.TestCase):
    blobdir = "/path/to/mount" # Path to the mounted container
    localdir = "/mnt/tmp" # A local temp directory, not the same one used by blobfuse.
    cachedir = "/mnt/blobfusetmp"
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
        if os.path.exists(self.localdir):
            shutil.rmtree(self.localdir);

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

    def test_WriteReadSingleFileUnicode(self):
        file1txt = "你好，世界！"
        filepath = os.path.join(self.blobstage, "文本: hello?world&we^are%all~together1 .txt");
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
        shutil.copytree(mediumBlobsSourceDir, localBlobDir)
        files = os.listdir(localBlobDir)
        self.assertEqual(10, len(files))

        mediumBlobsDestDir = os.path.join(self.blobstage, "medium")
        shutil.copytree(localBlobDir, mediumBlobsDestDir)
        files = os.listdir(mediumBlobsDestDir)
        self.assertEqual(10, len(files))

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
        except IOError as e:
            self.assertEqual(e.errno, errno.ENOENT) 

        # Directory not empty should throw
        with self.assertRaises(OSError):
            os.rmdir(testDir)

    def test_directory_operations(self):
        testDir = os.path.join(self.blobstage, "testDirectory")
        subdir1 = "subDir1"
        subdir2 = "Thİs!is-a%directory&name @1"
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
        testSubDir2 = os.path.join(testDir, subdir2)
        os.makedirs(testSubDir2)

        children = os.listdir(testDir);
        self.assertEqual(5, len(children))
        self.assertTrue("file1" in children)
        self.assertTrue("file2" in children)
        self.assertTrue("file3" in children)
        self.assertTrue(subdir1 in children)
        self.assertTrue(subdir2 in children)

        # Directory not empty should throw
        with self.assertRaises(OSError):
            os.rmdir(testDir)

        os.rmdir(testSubDir1)
        os.remove(os.path.join(testDir, "file2"))

        children = os.listdir(testDir)
        self.assertEqual(3, len(children))

        self.assertTrue("file1" in children)
        self.assertTrue("file3" in children)
        self.assertTrue(subdir2 in children)

        os.rmdir(testSubDir2)
        os.remove(os.path.join(testDir, "file1"))
        os.remove(os.path.join(testDir, "file3"))

        children = os.listdir(testDir)
        self.assertEqual(0, len(children))

        os.rmdir(testDir)
        self.assertFalse(os.path.exists(testDir))
        self.assertFalse(os.path.isdir(testDir))

    def test_symlink_operations(self):
        testSymlinkDir = os.path.join(self.blobstage, "test-symlink-directory")
        testSymlinkFile = os.path.join(self.blobstage, "test-symlink-file")
        testDir = os.path.join(self.blobstage, "test-dir")

        self.assertFalse(os.path.exists(testSymlinkDir))
        self.assertFalse(os.path.islink(testSymlinkDir))

        os.makedirs(testDir)
        os.symlink(testDir, testSymlinkDir)
        self.assertTrue(os.path.exists(testDir))
        self.assertTrue(os.path.islink(testSymlinkDir))
        
        filetxt = "Some file text here."
        testFilePath = os.path.join(testDir, "file1")
        with open(testFilePath, 'w') as fileblob:
            fileblob.write(filetxt)

        os.symlink(testFilePath, testSymlinkFile) 

        # test accessing data with the symlink directory path
        testFilewSymlinkPath = os.path.join(testSymlinkDir, "file1")
        self.assertTrue(os.path.exists(testFilewSymlinkPath))
        with open(testFilewSymlinkPath, 'r') as testFile:
            contents = testFile.read()
            self.assertEqual(filetxt, contents)

        # test accessing the same data with the symlink file path
        self.assertTrue(os.path.exists(testSymlinkFile))
        with open(testSymlinkFile, 'r') as testFile2:
            contents2 = testFile2.read()
            self.assertEqual(filetxt, contents2)

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

    def validate_dir_creation(self, dirpath, dirName, parentDir):
        os.stat(dirpath) # As long as this does not raise a FileNotFoundError, we are satisfied
                
        # Save values to move back to where we started and build testDir absolute path
        homeDir = os.getcwd()
        os.chdir(parentDir)
        parentDirAbsolute = os.getcwd()

        # Test that we can successfully move into the dir
        os.chdir(dirName)
        self.assertEqual(os.path.join(parentDirAbsolute, dirName), os.getcwd()) 

        # Test that we see the subdir when listing the current dir
        os.chdir("..")
        dir_entries = os.listdir()
        self.assertTrue(len(dir_entries) == 1)
        self.assertTrue(dirName in dir_entries)

        # Return to the test dir to continue with other tests
        os.chdir(homeDir)

    def test_make_new_directory(self):
        # Note that based on the value of self.blobdir, this also tests the relative path
        testDirName = "testDir"
        testDirPath = os.path.join(self.blobstage, testDirName)
        
        os.mkdir(testDirPath)

        self.validate_dir_creation(testDirPath, testDirName, self.blobstage)

        os.rmdir(os.path.join(self.blobstage, testDirName))

    def test_make_directory_name_exists(self):
        testDirName = "testDir"
        testDirPath = os.path.join(self.blobstage, testDirName)
        os.mkdir(testDirPath)

        with self.assertRaises(FileExistsError):
            os.mkdir(testDirPath)

        os.rmdir(os.path.join(self.blobstage, testDirName))

    # TODO: Validate on the client?  Or maybe just make it a known issue.  (Basically, we get a 400 here, instead of ENAMETOOLONG.)
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

    def test_make_directory_absolute_path(self):
        testDirName = "testDir"
        testDirAbsPath = os.path.join(os.getcwd(), self.blobstage, testDirName)

        os.mkdir(testDirAbsPath)

        self.validate_dir_creation(testDirAbsPath, testDirName, self.blobstage)

        os.rmdir(testDirAbsPath)

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
        
        self.assertTrue(len(entries) == 1) # Ensure we cannot see the .directory blob
        self.assertTrue(entries[0] == testFileName)

        os.remove(testFilePath)
        os.rmdir(testDirPath)

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
        self.assertTrue(len(entries) == 1) # Ensure we cannot see the .directory blob
        self.assertTrue(entries[0] == testSubdirName)

        os.rmdir(testSubdirPath)
        os.rmdir(testDirPath)


    def test_open_directory_dir_empty(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        os.mkdir(testDirPath)

        entries = os.listdir(testDirPath) # Note this also tests opening via relative path on existing directory
        self.assertTrue(len(entries) == 0)

        os.rmdir(testDirPath)

    def test_open_directory_dir_never_created(self):
        testDirName = "Test"
        testDirPath = os.path.join(self.blobstage, testDirName)

        with self.assertRaises(OSError) as e:
            os.listdir(testDirPath)

        self.assertEqual(e.exception.errno, errno.ENOENT)
        
    def test_open_directory_absolute_path(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)
        testDirAbs = os.path.abspath(testDirPath)

        os.mkdir(testDirAbs)

        entries = os.listdir(testDirAbs)
        self.assertTrue(len(entries) == 0)

        os.rmdir(testDirAbs)

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
        

    def test_remove_directory_empty(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        os.mkdir(testDirPath)

        os.rmdir(testDirPath) # Note this also tests removal via relative path

        self.validate_dir_removal(testDirPath, testDirName, self.blobstage)

    def test_remove_directory_absolute_path(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)
        testDirAbs = os.path.abspath(testDirPath)

        os.mkdir(testDirPath)

        os.rmdir(testDirAbs)

        self.validate_dir_removal(testDirAbs, testDirName, self.blobstage)

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

    def test_remove_directory_never_created(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        with self.assertRaises(OSError) as e:
            os.rmdir(testDirPath)
        self.assertEqual(e.exception.errno, errno.ENOENT)

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

    # TODO; Change to not call scandir(), which isn't available on Python 3.4 (which is common on centos)
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

            entries = os.scandir(testDirPath)
            entries.close()
            entries.close()

            os.remove(testFilePath)
            os.rmdir(testDirPath)
    '''

    # TODO: Investigate the behavior of 'mknod', especially because we haven't implemented it in FUSE.
    '''
    def test_fuse_except(self):
        # Run this test then run anything that should otherwise pass. Not sure what about the
        # failure in this test creates these phantom files. Maybe there's no cleanup in fuse upon exception
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        testFileName = "testFile"
        testFilePath = os.path.join(testDirPath, testFileName)

        os.mkdir(testDirPath)
        
        os.mknod(testFilePath, stat.S_IFREG | 0o777)
        os.access(testFilePath, os.F_OK)
        entries = os.listdir(testDirPath)

        self.assertTrue(len(entries) == 1)
        self.assertTrue(entries[0] == testFileName)

        os.remove(testFilePath)
        os.rmdir(testDirPath)
        
    '''

    '''
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
            testFile.close(testFile) # This line seems to make fuse crash

        testFile.close()
        entries = os.listdir(testDirPath)
        self.assertTrue(len(entries) == 2)
        self.assertTrue(testFileName in entries)
        self.assertTrue(testSubdirName in entries)

        os.remove(testFilePath)
        shutil.rmtree(testDirPath)
    '''
    def validate_file_creation(self, filePath, fileName, parentDir):
        os.stat(filePath) # As long as this doesn't fail, we are satisfied
        #print(os.stat(testFilePath))

        entries = os.listdir(parentDir)
        self.assertTrue(fileName in entries)

    def test_create_file_new_file(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY | os.O_TRUNC)
        #open(testFilePath, "w")

        self.validate_file_creation(testFilePath, testFileName, self.blobstage)

        os.close(fd)
        os.remove(testFilePath)


    def test_create_file_name_conflict_file(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT)

        with self.assertRaises(OSError) as e:
            os.open(testFilePath, os.O_EXCL | os.O_CREAT)
        self.assertEqual(e.exception.errno, errno.EEXIST)

        os.close(fd)
        os.remove(testFilePath)

    def test_create_file_name_conflict_dir(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        os.mkdir(testFilePath)
        with self.assertRaises(OSError) as e:
            os.open(testFilePath, os.O_CREAT | os.O_EXCL)
        self.assertEqual(e.exception.errno, errno.EEXIST)

        os.rmdir(testFilePath)

    def test_open_file_nonexistant_file_read(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        with self.assertRaises(OSError) as e:
            os.open(testFilePath, os.O_RDONLY)

        self.assertEqual(e.exception.errno, errno.ENOENT)

    def test_open_file_nonexistant_file_write(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        with self.assertRaises(OSError) as e:
            os.open(testFilePath, os.O_WRONLY)
            
        self.assertEqual(e.exception.errno, errno.ENOENT)

    def test_open_file_exists_read_write_empty(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY) # This covers opening a file that exists and is empty
        testData = "Test data"
        os.write(fd, testData.encode())
        os.close(fd) # It would be possible to seek, but that should be a separate test. Reoping is more granular

        fd = os.open(testFilePath, os.O_CREAT | os.O_RDONLY)
        data = os.read(fd, 15)
        self.assertEqual(data.decode(), testData)
        
        os.close(fd)
        os.remove(testFilePath)

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

    def close_file_func(self, filePath):
        fd = os.open(filePath, os.O_RDONLY)
        os.close(fd)

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

    def test_close_file(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY)
        os.close(fd)

        with self.assertRaises(OSError) as e:
            os.write(fd, "data".encode())
        self.assertEqual(e.exception.errno, errno.EBADF)

        os.remove(testFilePath)

    def validate_file_removal(self, filePath, fileName, parentDir):
        with self.assertRaises(OSError) as e:
            os.stat(filePath)
        self.assertEqual(e.exception.errno, errno.ENOENT)

        entries = os.listdir(parentDir)
        self.assertFalse(fileName in entries)

    def test_remove_file(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        f = os.open(testFilePath, os.O_CREAT)
        os.close(f)
        os.remove(testFilePath)

        self.validate_file_removal(testFilePath, testFileName, self.blobstage)

    def test_remove_file_still_open(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        f = os.open(testFilePath, os.O_CREAT)
        os.remove(testFilePath)

        self.validate_file_removal(testFilePath, testFileName, self.blobstage)

    def test_truncate_file(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY)
        os.write(fd, "random data".encode())
        os.ftruncate(fd, 0)
        os.close(fd)

        self.assertEqual(os.stat(testFilePath).st_size, 0)

        os.remove(testFilePath)

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


    def test_stat_file(self):
        testFileName = "TestFile"
        testFilePath = os.path.join(self.blobstage, testFileName)

        #fd = os.open(testFilePath, os.O_CREAT | os.O_WRONLY)
        f = open(testFilePath, "w")
        testData = "test data"
        #os.write(fd, testData.encode())
        f.write(testData)
        f.close()

        self.assertEqual(os.stat(testFilePath).st_size, len(testData))

        os.remove(testFilePath)

    def test_stat_dir(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)

        os.mkdir(testDirPath)
        self.assertEqual(os.stat(testDirPath).st_size, 4096) # The minimum directory size on Linux

        os.rmdir(testDirPath)

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


    def test_rename_dir_nonempty(self):
        testDirName = "TestDir"
        testDirPath = os.path.join(self.blobstage, testDirName)
        testFileName = "testFile"
        testFilePath = os.path.join(testDirPath, testFileName)
        destDirName = "NewName"
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

    # TODO: implement flock
    '''
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
    def read_file_func(self, filePath, start, end, testData):
        fd = os.open(filePath, os.O_RDONLY)
        os.lseek(fd, start, os.SEEK_SET)
        data = os.read(fd, end-start)
        self.assertEqual(data.decode(), testData[start:end])
        os.close(fd)

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

    def write_file_func(self, filePath, data):
        fd = os.open(filePath, os.O_WRONLY | os.O_APPEND)
        os.write(fd, data.encode())
        os.close(fd)

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
        self.assertEqual(time_of_upload, blob_last_modified)

        # prime the cache and check the attributes again
        fd = os.open(testFilePath, os.O_RDONLY)

        # check whether getattr from the cache is working
        fileowner = os.stat(testFilePath).st_uid
        filegroup = os.stat(testFilePath).st_gid
        file_last_modified = os.stat(testFilePath).st_mtime

        self.assertEqual(fileowner, os.getuid())
        self.assertEqual(filegroup, os.getgid())
        self.assertEqual(time_of_upload, file_last_modified)
        
        os.close(fd)
        os.remove(testFilePath)


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
        for i in range(0,20):
            filename = str(uuid.uuid4())
            filepath = os.path.join(mediumBlobsSourceDir, filename)
            os.system("head -c 100M < /dev/zero >> " + filepath);

        # This removes the cached entries of the files just created, so they are on the service but not local.
        # This will force each thread to call ensure_directory_exists_in_cache when trying to access its file.
        shutil.rmtree(self.cachedir + '/root/testing/' + sourceDirName)

        threads = []

        for filename in os.listdir(mediumBlobsSourceDir):
            path = os.path.join(mediumBlobsSourceDir, filename)
            threads.append(threading.Thread(target=self.read_file_func, args=(path, 0, 0, "",))) # Note that read_file_func also opens the file, which is the desired behavior

        for thread in threads:
            thread.start()

        for thread in threads:
            thread.join()

        shutil.rmtree(mediumBlobsSourceDir)

        
if __name__ == '__main__':
    unittest.main()



