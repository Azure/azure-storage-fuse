package distributed_cache

import (
	"os"
	"reflect"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/component/azstorage"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	gomock "github.com/golang/mock/gomock"
)

var _ azstorage.AzConnection = &MockStorage{}

// MockStorage simulates azstorage.AzStorage.
type MockStorage struct {
	ctrl     *gomock.Controller
	recorder *MockComponentMockRecorder
}

type MockComponentMockRecorder struct {
	mock *MockStorage
}

func (mr *MockComponentMockRecorder) GetAttr(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAttr", reflect.TypeOf((*MockStorage)(nil).GetAttr), arg0)
}

// NewMockComponent creates a new mock instance.
func NewMockStroage(ctrl *gomock.Controller) *MockStorage {
	mock := &MockStorage{ctrl: ctrl}
	mock.recorder = &MockComponentMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStorage) EXPECT() *MockComponentMockRecorder {
	return m.recorder
}

func (m *MockStorage) Configure(cfg azstorage.AzStorageConfig) error {
	return nil
}
func (m *MockStorage) UpdateConfig(cfg azstorage.AzStorageConfig) error {
	return nil
}

func (m *MockStorage) SetupPipeline() error {
	return nil
}
func (m *MockStorage) TestPipeline() error {
	return nil
}

func (m *MockStorage) ListContainers() ([]string, error) {
	return []string{}, nil
}

func (m *MockStorage) SetPrefixPath(string) error {
	return nil
}
func (m *MockStorage) CreateFile(name string, mode os.FileMode) error {
	return nil
}
func (m *MockStorage) CreateDirectory(name string, etag bool) error {
	return nil
}
func (m *MockStorage) CreateLink(source string, target string) error {
	return nil
}

func (m *MockStorage) DeleteFile(name string) error {
	return nil
}
func (m *MockStorage) DeleteDirectory(name string) error {
	return nil
}

func (m *MockStorage) RenameFile(string, string, *internal.ObjAttr) error {
	return nil
}
func (m *MockStorage) RenameDirectory(string, string) error {
	return nil
}

func (m *MockStorage) GetAttr(name string) (attr *internal.ObjAttr, err error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAttr", name)
	ret1, _ := ret[1].(error)
	ret0, _ := ret[0].(*internal.ObjAttr)
	return ret0, ret1

}

func (m *MockStorage) List(prefix string, marker *string, count int32) ([]*internal.ObjAttr, *string, error) {
	return nil, nil, nil
}

func (m *MockStorage) ReadToFile(name string, offset int64, count int64, fi *os.File) error {
	return nil
}
func (m *MockStorage) ReadBuffer(name string, offset int64, len int64) ([]byte, error) {
	return nil, nil
}
func (m *MockStorage) ReadInBuffer(name string, offset int64, len int64, data []byte, etag *string) error {
	return nil
}

func (m *MockStorage) WriteFromFile(name string, metadata map[string]*string, fi *os.File) error {
	return nil
}
func (m *MockStorage) WriteFromBuffer(options internal.WriteFromBufferOptions) error {
	return nil
}
func (m *MockStorage) Write(options internal.WriteFileOptions) error {
	return nil
}
func (m *MockStorage) GetFileBlockOffsets(name string) (*common.BlockOffsetList, error) {
	return nil, nil
}

func (m *MockStorage) ChangeMod(string, os.FileMode) error {
	return nil
}
func (m *MockStorage) ChangeOwner(string, int, int) error {
	return nil
}
func (m *MockStorage) TruncateFile(string, int64) error {
	return nil
}
func (m *MockStorage) StageAndCommit(name string, bol *common.BlockOffsetList) error {
	return nil
}

func (m *MockStorage) GetCommittedBlockList(string) (*internal.CommittedBlockList, error) {
	return nil, nil
}
func (m *MockStorage) StageBlock(string, []byte, string) error {
	return nil
}
func (m *MockStorage) CommitBlocks(string, []string, *string) error {
	return nil
}

func (m *MockStorage) UpdateServiceClient(_, _ string) error {
	return nil
}

func (m *MockStorage) SetFilter(string) error {
	return nil
}
