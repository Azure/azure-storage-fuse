package comp

import (
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/component/loopback"
	xinternal "github.com/Azure/azure-storage-fuse/v2/component/xload/internal"
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
	rdm, err := NewRemoteDataManager(nil, nil)
	suite.assert.NotNil(err)
	suite.assert.Nil(rdm)
	suite.assert.Contains(err.Error(), "invalid parameters sent to create remote data manager")

	remote := loopback.NewLoopbackFSComponent()
	statsMgr, err := xinternal.NewStatsManager(1, false)
	suite.assert.Nil(err)
	suite.assert.NotNil(statsMgr)

	rdm, err = NewRemoteDataManager(remote, statsMgr)
	suite.assert.Nil(err)
	suite.assert.NotNil(rdm)
}

func TestDatamanagerSuite(t *testing.T) {
	suite.Run(t, new(dataManagerTestSuite))
}
