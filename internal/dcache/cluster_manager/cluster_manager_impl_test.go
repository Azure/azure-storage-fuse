package clustermanager

import (
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ClusterManagerImplTestSuite struct {
	suite.Suite
	cmi ClusterManagerImpl
}

func (suite *ClusterManagerImplTestSuite) TestCheckIfClusterMapExists() {
	orig := getClusterMap
	defer func() { getClusterMap = orig }()

	// 1) success
	getClusterMap = func() error { return nil }
	exists, err := suite.cmi.checkIfClusterMapExists()
	suite.NoError(err)
	suite.True(exists)

	// 2) os.ErrNotExist
	getClusterMap = func() error { return os.ErrNotExist }
	exists, err = suite.cmi.checkIfClusterMapExists()
	suite.NoError(err)
	suite.False(exists)

	// 3) syscall.ENOENT
	getClusterMap = func() error { return syscall.ENOENT }
	exists, err = suite.cmi.checkIfClusterMapExists()
	suite.NoError(err)
	suite.False(exists)

	// 4) other error
	testErr := errors.New("boom")
	getClusterMap = func() error { return testErr }
	exists, err = suite.cmi.checkIfClusterMapExists()
	suite.EqualError(err, "boom")
	suite.False(exists)
}

func TestClusterManagerImpl(t *testing.T) {
	suite.Run(t, new(ClusterManagerImplTestSuite))
}
