//go:build !unittest
// +build !unittest

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package e2e_tests

import "os"

func (suite *dataValidationTestSuite) TestShrinkExistingFile() {
	fileName := "shrink_existing_file"
	localFilePath, remoteFilePath := convertFileNameToFilePath(fileName)

	t := func(initSize int64, shrinkSize int64) {
		suite.helperCreateFile(localFilePath, remoteFilePath, initSize)
		suite.helperValidateFileContent(localFilePath, remoteFilePath)
		suite.helperTruncateFile(localFilePath, remoteFilePath, shrinkSize)
		suite.helperValidateFileContent(localFilePath, remoteFilePath)
		suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, tObj.testCachePath})
	}

	t(1024*1024, 512*1024)
	t(8*1024*1024, 4*1024*1024)
	t(32*1024*1024, 16*1024*1024)
	t(32*1024*1024+16, 16*1024*1024-9)
	t(1*1024*1024*1024, 512*1024*1024-18)
	t(10*1024*1024*1024, 1*1024*1024-8)
}

func (suite *dataValidationTestSuite) TestExpandExistingFile() {
	fileName := "expand_existing_file"
	localFilePath, remoteFilePath := convertFileNameToFilePath(fileName)

	t := func(initSize int64, expandSize int64) {
		suite.helperCreateFile(localFilePath, remoteFilePath, initSize)
		suite.helperValidateFileContent(localFilePath, remoteFilePath)
		suite.helperTruncateFile(localFilePath, remoteFilePath, expandSize)
		suite.helperValidateFileContent(localFilePath, remoteFilePath)
		suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, tObj.testCachePath})
	}

	t(1024*1024, 2*1024*1024)
	t(8*1024*1024, 16*1024*1024)
	t(8*1024*1024-1, 16*1024*1024+18)
	t(16*1024*1024-1, 256*1024*1024+18)
	t(1*1024*1024*1024, 2*1024*1024*1024+16)
	t(1*1024*1024, 10*1024*1024*1024+8)
}

func (suite *dataValidationTestSuite) TestTruncateNonExistingFile() {
	fileName := "truncate_non_existing_file"
	localFilePath, remoteFilePath := convertFileNameToFilePath(fileName)

	lErr := os.Truncate(localFilePath, 1024*1024)
	suite.Error(lErr)

	rErr := os.Truncate(remoteFilePath, 1024*1024)
	suite.Error(rErr)

	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, tObj.testCachePath})
}

func (suite *dataValidationTestSuite) TestWriteBeforeTruncate() {
	fileName := "write_before_truncate"
	localFilePath, remoteFilePath := convertFileNameToFilePath(fileName)

	suite.helperCreateFile(localFilePath, remoteFilePath, 1024*1024)
	suite.helperWriteToFile(localFilePath, remoteFilePath, 512*1024, 512*1024)
	suite.helperTruncateFile(localFilePath, remoteFilePath, 512*1024)
	suite.helperValidateFileContent(localFilePath, remoteFilePath)
	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, tObj.testCachePath})
}

func (suite *dataValidationTestSuite) TestWriteAfterTruncate() {
	fileName := "write_after_truncate"
	localFilePath, remoteFilePath := convertFileNameToFilePath(fileName)

	suite.helperCreateFile(localFilePath, remoteFilePath, 0)
	suite.helperTruncateFile(localFilePath, remoteFilePath, 512*1024)
	suite.helperWriteToFile(localFilePath, remoteFilePath, 512*1024, 512*1024)
	suite.helperValidateFileContent(localFilePath, remoteFilePath)
	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, tObj.testCachePath})
}

func (suite *dataValidationTestSuite) TestTruncateToZero() {
	fileName := "truncate_to_zero"
	localFilePath, remoteFilePath := convertFileNameToFilePath(fileName)

	suite.helperCreateFile(localFilePath, remoteFilePath, 1024*1024)
	suite.helperTruncateFile(localFilePath, remoteFilePath, 0)
	suite.helperValidateFileContent(localFilePath, remoteFilePath)
	suite.dataValidationTestCleanup([]string{localFilePath, remoteFilePath, tObj.testCachePath})
}
