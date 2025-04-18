package clustermanager

import (
	"os"
	"syscall"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	mm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/metadata_manager"
	"github.com/stretchr/testify/suite"
)

type clusterManagerImplTestSuite struct {
	suite.Suite
	mockStorage         dcache.StorageCallbacks
	mockMetaDataManager *mm.MockMetaDataManager
	cmi                 ClusterManagerImpl
}

func (s *clusterManagerImplTestSuite) SetupTest() {
	// Create the mock MetadataManager
	s.mockMetaDataManager = mm.NewMockMetaDataManager()

	// Assign it to clusterManagerImpl
	s.cmi = ClusterManagerImpl{
		metaDataManager: s.mockMetaDataManager,
	}
}

func (s *clusterManagerImplTestSuite) TestCheckIfClusterMapExists_FileExist() {
	// Arrange
	mockCacheID := "testCacheId"
	// Configure the mock to return a file-not-found error
	s.mockMetaDataManager.On("GetClusterMap").Return([]byte{}, nil, nil)

	// Act
	exists, err := s.cmi.checkIfClusterMapExists(mockCacheID)

	// Assert
	s.NoError(err)
	s.True(exists)
	s.mockMetaDataManager.AssertCalled(s.T(), "GetClusterMap")
}

func (s *clusterManagerImplTestSuite) TestCheckIfClusterMapExists_FileDoesNotExist() {
	// Arrange
	mockCacheID := "testCacheId"
	// Configure the mock to return a file-not-found error
	s.mockMetaDataManager.On("GetClusterMap").Return([]byte{}, nil, os.ErrNotExist)

	// Act
	exists, err := s.cmi.checkIfClusterMapExists(mockCacheID)

	// Assert
	s.NoError(err)
	s.False(exists)
	s.mockMetaDataManager.AssertCalled(s.T(), "GetClusterMap")
}

func (s *clusterManagerImplTestSuite) TestCheckIfClusterMapExists_UnexpectedError() {
	// Arrange
	mockCacheID := "testCacheId"
	mockError := syscall.EIO
	s.mockMetaDataManager.On("GetClusterMap").Return([]byte{}, nil, mockError)

	// Act
	exists, err := s.cmi.checkIfClusterMapExists(mockCacheID)

	// Assert
	s.Error(err)
	s.False(exists)
	s.Equal(mockError, err)
	s.mockMetaDataManager.AssertCalled(s.T(), "GetClusterMap")
}

func TestClusterManagerImpl(t *testing.T) {
	suite.Run(t, new(clusterManagerImplTestSuite))
}
