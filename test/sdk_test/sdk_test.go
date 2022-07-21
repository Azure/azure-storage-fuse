// +build !unittest
/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"time"

	"errors"
	"flag"
	"testing"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

var accountName string
var sas string
var containerName string
var filepath string

func TestMain(m *testing.M) {

	paccountName := flag.String("account", "", "Account name to be used")
	psas := flag.String("sas", "", "Storage sas for the account")
	pcontainerName := flag.String("container", "", "Container name for the account")
	pfilepath := flag.String("prefix", "", "File Prefix for the operation")
	flag.Parse()

	//fmt.Println(*paccountName, " : ", *psas, " : ", *pcontainerName, " : ", *pfilepath)
	accountName = *paccountName
	sas = *psas
	containerName = *pcontainerName
	filepath = *pfilepath

	if accountName == "" || sas == "" || containerName == "" || filepath == "" {
		fmt.Println("Kindly provide the parameters...")
		fmt.Println("Usage : sdk_test.go -account=<account name> -sas=<storage sas> -container=<container name> -prefix=<file prefix>")
		panic(errors.New("Invalid arguments"))
	}

	m.Run()
}

func TestDownloadUpload(t *testing.T) {

	c := azblob.NewAnonymousCredential()
	p := azblob.NewPipeline(c, azblob.PipelineOptions{})
	cURL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s%s", accountName, containerName, sas))
	containerURL := azblob.NewContainerURL(*cURL, p)

	for i := 1; i < 7; i++ {
		// Generate the url
		blobname := fmt.Sprintf("%s%d", filepath, i)
		//filename := fmt.Sprintf("%s%s", "/mnt/ramdisk/", blobname)
		filename := fmt.Sprintf("%s%s", "/mnt/ramdisk/", blobname)
		blobURL := containerURL.NewBlockBlobURL(path.Base(blobname))

		t.Log("----------------------------------------------------------------------")
		t.Log("Next test file ", filename)
		// Download the file
		file, err := os.Create(filename)
		if err != nil {
			panic(err)
		}

		t.Log("download : ", filename)
		time1 := time.Now()
		err = azblob.DownloadBlobToFile(
			context.Background(),
			blobURL.BlobURL,
			0, 0,
			file,
			azblob.DownloadFromBlobOptions{
				BlockSize:   8 * 1024 * 1024,
				Parallelism: 64,
			})
		if err != nil {
			t.Log(err.Error())
		}
		time2 := time.Now()
		size, _ := file.Seek(0, io.SeekEnd)

		t.Log("download done : ", filename, " size : ", size)

		diff := time2.Sub(time1).Seconds()
		t.Log("Time taken to Download ", filename, "is ", diff, " Seconds")
		file.Close()

		t.Log("----------------------------------------------------------------------")
		// Upload the file
		file, err = os.Open(filename)
		if err != nil {
			panic(err)
		}
		t.Log("upload : ", filename)

		time1 = time.Now()
		_, err = azblob.UploadFileToBlockBlob(
			context.Background(),
			file,
			blobURL,
			azblob.UploadToBlockBlobOptions{
				BlockSize:   8 * 1024 * 1024,
				Parallelism: 64,
			})
		if err != nil {
			t.Log(err.Error())
		}

		time2 = time.Now()
		t.Log("upload done : ", filename)

		diff = time2.Sub(time1).Seconds()
		t.Log("Time taken to Upload ", filename, "is ", diff, " Seconds")
		file.Close()

		_ = os.Remove(filename)
	}
}
