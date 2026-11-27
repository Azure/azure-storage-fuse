// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package xload

import (
	"context"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/component/loopback"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type dataManagerTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *dataManagerTestSuite) SetupSuite() {
	suite.assert = assert.New(suite.T())
}

func (suite *dataManagerTestSuite) TestNewRemoteDataManager() {
	rdm, err := newRemoteDataManager(nil)
	suite.assert.Error(err)
	suite.assert.Nil(rdm)
	suite.assert.Contains(err.Error(), "invalid parameters sent to create remote data manager")

	rdm, err = newRemoteDataManager(&remoteDataManagerOptions{})
	suite.assert.Error(err)
	suite.assert.Nil(rdm)
	suite.assert.Contains(err.Error(), "invalid parameters sent to create remote data manager")

	remote := loopback.NewLoopbackFSComponent()
	statsMgr, err := NewStatsManager(1, false, nil)
	suite.assert.NoError(err)
	suite.assert.NotNil(statsMgr)

	rdm, err = newRemoteDataManager(&remoteDataManagerOptions{
		workerCount: 4,
		remote:      remote,
		statsMgr:    statsMgr,
	})
	suite.assert.NoError(err)
	suite.assert.NotNil(rdm)
}

func (suite *dataManagerTestSuite) TestProcessErrors() {
	rdm := &remoteDataManager{}

	ctx, cancel := context.WithCancel(context.Background())
	item := &WorkItem{
		CompName: DATA_MANAGER,
		Path:     "test",
		Block:    &Block{},
		Download: false,
		Ctx:      ctx,
	}

	dataLength, err := rdm.Process(item)
	suite.assert.Error(err)
	suite.assert.Equal(0, dataLength)

	// cancel the context
	cancel()

	dataLength, err = rdm.Process(item)
	suite.assert.Error(err)
	suite.assert.Equal(0, dataLength)
}

func TestDatamanagerSuite(t *testing.T) {
	suite.Run(t, new(dataManagerTestSuite))
}
