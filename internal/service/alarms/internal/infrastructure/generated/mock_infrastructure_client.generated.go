// Code generated by MockGen. DO NOT EDIT.
// Source: infrastructure.go
//
// Generated by this command:
//
//	mockgen -source=infrastructure.go -destination=generated/mock_infrastructure_client.generated.go -package=generated
//
// Package generated is a generated GoMock package.
package generated

import (
	context "context"
	reflect "reflect"

	uuid "github.com/google/uuid"
	gomock "go.uber.org/mock/gomock"
)

// MockClient is a mock of Client interface.
type MockClient struct {
	ctrl     *gomock.Controller
	recorder *MockClientMockRecorder
}

// MockClientMockRecorder is the mock recorder for MockClient.
type MockClientMockRecorder struct {
	mock *MockClient
}

// NewMockClient creates a new mock instance.
func NewMockClient(ctrl *gomock.Controller) *MockClient {
	mock := &MockClient{ctrl: ctrl}
	mock.recorder = &MockClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClient) EXPECT() *MockClientMockRecorder {
	return m.recorder
}

// FetchAll mocks base method.
func (m *MockClient) FetchAll(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchAll", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// FetchAll indicates an expected call of FetchAll.
func (mr *MockClientMockRecorder) FetchAll(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchAll", reflect.TypeOf((*MockClient)(nil).FetchAll), arg0)
}

// GetAlarmDefinitionID mocks base method.
func (m *MockClient) GetAlarmDefinitionID(ctx context.Context, ObjectTypeID uuid.UUID, name, severity string) (uuid.UUID, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAlarmDefinitionID", ctx, ObjectTypeID, name, severity)
	ret0, _ := ret[0].(uuid.UUID)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAlarmDefinitionID indicates an expected call of GetAlarmDefinitionID.
func (mr *MockClientMockRecorder) GetAlarmDefinitionID(ctx, ObjectTypeID, name, severity any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAlarmDefinitionID", reflect.TypeOf((*MockClient)(nil).GetAlarmDefinitionID), ctx, ObjectTypeID, name, severity)
}

// GetObjectTypeID mocks base method.
func (m *MockClient) GetObjectTypeID(ctx context.Context, objectID uuid.UUID) (uuid.UUID, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetObjectTypeID", ctx, objectID)
	ret0, _ := ret[0].(uuid.UUID)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetObjectTypeID indicates an expected call of GetObjectTypeID.
func (mr *MockClientMockRecorder) GetObjectTypeID(ctx, objectID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetObjectTypeID", reflect.TypeOf((*MockClient)(nil).GetObjectTypeID), ctx, objectID)
}

// Name mocks base method.
func (m *MockClient) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockClientMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockClient)(nil).Name))
}

// Setup mocks base method.
func (m *MockClient) Setup() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Setup")
	ret0, _ := ret[0].(error)
	return ret0
}

// Setup indicates an expected call of Setup.
func (mr *MockClientMockRecorder) Setup() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Setup", reflect.TypeOf((*MockClient)(nil).Setup))
}

// Sync mocks base method.
func (m *MockClient) Sync(ctx context.Context) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Sync", ctx)
}

// Sync indicates an expected call of Sync.
func (mr *MockClientMockRecorder) Sync(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Sync", reflect.TypeOf((*MockClient)(nil).Sync), ctx)
}
