// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2022 Marvell International Ltd.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
)

func TestFrontEnd_CreateNvmeSubsystem(t *testing.T) {
	spec := &pb.NvmeSubsystemSpec{
		Nqn:          "nqn.2022-09.io.spdk:opi3",
		SerialNumber: "OpiSerialNumber",
		ModelNumber:  "OpiModelNumber",
	}
	t.Cleanup(utils.CheckTestProtoObjectsNotChanged(spec)(t, t.Name()))
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))

	tests := map[string]struct {
		id      string
		in      *pb.NvmeSubsystem
		out     *pb.NvmeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		exist   bool
	}{
		"illegal resource_id": {
			id: "CapitalLettersNotAllowed",
			in: &pb.NvmeSubsystem{
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			exist:   false,
		},
		"valid request with invalid SPDK response": {
			id: testSubsystemID,
			in: &pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result": {"status": 1}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not create NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			exist:   false,
		},
		"valid request with empty SPDK response": {
			id: testSubsystemID,
			in: &pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_create_subsystem: %v", "EOF"),
			exist:   false,
		},
		"valid request with ID mismatch SPDK response": {
			id: testSubsystemID,
			in: &pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_create_subsystem: %v", "json response ID mismatch"),
			exist:   false,
		},
		"valid request with error code from SPDK response": {
			id: testSubsystemID,
			in: &pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_create_subsystem: %v", "json response error: myopierr"),
			exist:   false,
		},
		"valid request with error code from SPDK version response": {
			id: testSubsystemID,
			in: &pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("spdk_get_version: %v", "json response error: myopierr"),
			exist:   false,
		},
		"valid request with valid SPDK response": {
			id: testSubsystemID,
			in: &pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: spec,
			},
			out: &pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: spec,
				Status: &pb.NvmeSubsystemStatus{
					FirmwareRevision: "SPDK v20.10",
				},
			},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`, `{"jsonrpc":"2.0","id":%d,"result":{"version":"SPDK v20.10","fields":{"major":20,"minor":10,"patch":0,"suffix":""}}}`},
			errCode: codes.OK,
			errMsg:  "",
			exist:   false,
		},
		"already exists": {
			id: testSubsystemID,
			in: &pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: spec,
			},
			out:     &testSubsystemWithStatus,
			spdk:    []string{},
			errCode: codes.OK,
			errMsg:  "",
			exist:   true,
		},
		"no required field": {
			id:      testControllerID,
			in:      nil,
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: nvme_subsystem",
			exist:   false,
		},
		"too long nqn field": {
			id: testControllerID,
			in: &pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: &pb.NvmeSubsystemSpec{
					Nqn:          strings.Repeat("a", 224),
					SerialNumber: strings.Repeat("b", 20),
					ModelNumber:  strings.Repeat("c", 40),
				},
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Nqn value (%s) is too long, have to be between 1 and %d", strings.Repeat("a", 224), 223),
			exist:   false,
		},
		"too long model field": {
			id: testControllerID,
			in: &pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: &pb.NvmeSubsystemSpec{
					Nqn:          strings.Repeat("a", 223),
					SerialNumber: strings.Repeat("b", 20),
					ModelNumber:  strings.Repeat("c", 41),
				},
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("ModelNumber value (%s) is too long, have to be between 1 and %d", strings.Repeat("c", 41), 40),
			exist:   false,
		},
		"too long serial field": {
			id: testControllerID,
			in: &pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: &pb.NvmeSubsystemSpec{
					Nqn:          strings.Repeat("a", 223),
					SerialNumber: strings.Repeat("b", 21),
					ModelNumber:  strings.Repeat("c", 40),
				},
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("SerialNumber value (%s) is too long, have to be between 1 and %d", strings.Repeat("b", 21), 20),
			exist:   false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			_ = testEnv.opiSpdkServer.store.Set(testControllerName, &testControllerWithStatus)
			_ = testEnv.opiSpdkServer.store.Set(testNamespaceName, &testNamespaceWithStatus)
			if tt.exist {
				_ = testEnv.opiSpdkServer.store.Set(testSubsystemName, &testSubsystemWithStatus)
				// testEnv.opiSpdkServer.Subsystems[testSubsystemID].Spec.Id = &pc.ObjectKey{Value: testSubsystemID}
			}
			if tt.out != nil {
				tt.out = utils.ProtoClone(tt.out)
				tt.out.Name = testSubsystemName
			}

			request := &pb.CreateNvmeSubsystemRequest{NvmeSubsystem: tt.in, NvmeSubsystemId: tt.id}
			response, err := testEnv.client.CreateNvmeSubsystem(testEnv.ctx, request)

			if !proto.Equal(response, tt.out) {
				t.Error("response: expected", tt.out, "received", response)
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}
		})
	}
}

func TestFrontEnd_DeleteNvmeSubsystem(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"valid request with invalid SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not delete NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			missing: false,
		},
		"valid request with empty SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_delete_subsystem: %v", "EOF"),
			missing: false,
		},
		"valid request with ID mismatch SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_delete_subsystem: %v", "json response ID mismatch"),
			missing: false,
		},
		"valid request with error code from SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_delete_subsystem: %v", "json response error: myopierr"),
			missing: false,
		},
		"valid request with valid SPDK response": {
			in:      testSubsystemName,
			out:     &emptypb.Empty{},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`},
			errCode: codes.OK,
			errMsg:  "",
			missing: false,
		},
		"valid request with unknown key": {
			in:      utils.ResourceIDToSubsystemName("unknown-subsystem-id"),
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", utils.ResourceIDToSubsystemName("unknown-subsystem-id")),
			missing: false,
		},
		"unknown key with missing allowed": {
			in:      "unknown-id",
			out:     &emptypb.Empty{},
			spdk:    []string{},
			errCode: codes.OK,
			errMsg:  "",
			missing: true,
		},
		"malformed name": {
			in:      "-ABC-DEF",
			out:     &emptypb.Empty{},
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			missing: false,
		},
		"no required field": {
			in:      "",
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: name",
			missing: false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			_ = testEnv.opiSpdkServer.store.Set(testSubsystemName, &testSubsystemWithStatus)
			_ = testEnv.opiSpdkServer.store.Set(testControllerName, &testControllerWithStatus)
			_ = testEnv.opiSpdkServer.store.Set(testNamespaceName, &testNamespaceWithStatus)

			request := &pb.DeleteNvmeSubsystemRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNvmeSubsystem(testEnv.ctx, request)

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}

			if reflect.TypeOf(response) != reflect.TypeOf(tt.out) {
				t.Error("response: expected", reflect.TypeOf(tt.out), "received", reflect.TypeOf(response))
			}
		})
	}
}

func TestFrontEnd_UpdateNvmeSubsystem(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		mask    *fieldmaskpb.FieldMask
		in      *pb.NvmeSubsystem
		out     *pb.NvmeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"invalid fieldmask": {
			mask: &fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
			in: &pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: testSubsystem.Spec,
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
		},
		"unimplemented method": {
			mask: nil,
			in: &pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: testSubsystem.Spec,
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unimplemented,
			errMsg:  fmt.Sprintf("%v method is not implemented", "UpdateNvmeSubsystem"),
		},
		"valid request with unknown key": {
			mask: nil,
			in: &pb.NvmeSubsystem{
				Name: utils.ResourceIDToSubsystemName("unknown-subsystem-id"),
				Spec: &pb.NvmeSubsystemSpec{
					Nqn: "nqn.2022-09.io.spdk:opi3",
				},
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", utils.ResourceIDToSubsystemName("unknown-subsystem-id")),
		},
		"malformed name": {
			mask: nil,
			in: &pb.NvmeSubsystem{
				Name: "-ABC-DEF",
				Spec: testSubsystem.Spec,
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			_ = testEnv.opiSpdkServer.store.Set(testSubsystemName, &testSubsystemWithStatus)
			_ = testEnv.opiSpdkServer.store.Set(testControllerName, &testControllerWithStatus)
			_ = testEnv.opiSpdkServer.store.Set(testNamespaceName, &testNamespaceWithStatus)

			request := &pb.UpdateNvmeSubsystemRequest{NvmeSubsystem: tt.in, UpdateMask: tt.mask}
			response, err := testEnv.client.UpdateNvmeSubsystem(testEnv.ctx, request)

			if !proto.Equal(response, tt.out) {
				t.Error("response: expected", tt.out, "received", response)
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}
		})
	}
}

func TestFrontEnd_ListNvmeSubsystem(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		out     []*pb.NvmeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		size    int32
		token   string
	}{
		"valid request with invalid SPDK response": {
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not list %v", "subsystems"),
			size:    0,
			token:   "",
		},
		"valid request with empty SPDK response": {
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "EOF"),
			size:    0,
			token:   "",
		},
		"valid request with ID mismatch SPDK response": {
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "json response ID mismatch"),
			size:    0,
			token:   "",
		},
		"valid request with error code from SPDK response": {
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "json response error: myopierr"),
			size:    0,
			token:   "",
		},
		"pagination negative": {
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  "negative PageSize is not allowed",
			size:    -10,
			token:   "",
		},
		"pagination error": {
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			size:    0,
			token:   "unknown-pagination-token",
		},
		"pagination": {
			out: []*pb.NvmeSubsystem{
				{Spec: &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi1"}},
			},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "subsys_list": [{"subnqn": "nqn.2022-09.io.spdk:opi1"},{"subnqn": "nqn.2022-09.io.spdk:opi2"},{"subnqn": "nqn.2022-09.io.spdk:opi3"}]}}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    1,
			token:   "",
		},
		"pagination overflow": {
			out: []*pb.NvmeSubsystem{
				{Spec: &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi1"}},
				{Spec: &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi2"}},
				{Spec: &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi3"}},
			},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "subsys_list": [{"subnqn": "nqn.2022-09.io.spdk:opi1"},{"subnqn": "nqn.2022-09.io.spdk:opi2"},{"subnqn": "nqn.2022-09.io.spdk:opi3"}]}}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    1000,
			token:   "",
		},
		"pagination offset": {
			out: []*pb.NvmeSubsystem{
				{Spec: &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi2"}},
			},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "subsys_list": [{"subnqn": "nqn.2022-09.io.spdk:opi1"},{"subnqn": "nqn.2022-09.io.spdk:opi2"},{"subnqn": "nqn.2022-09.io.spdk:opi3"}]}}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    1,
			token:   "existing-pagination-token",
		},
		"valid request with valid SPDK response": {
			out: []*pb.NvmeSubsystem{
				{Spec: &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi1"}},
				{Spec: &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi2"}},
				{Spec: &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi3"}},
			},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "subsys_list": [{"subnqn": "nqn.2022-09.io.spdk:opi1"},{"subnqn": "nqn.2022-09.io.spdk:opi2"},{"subnqn": "nqn.2022-09.io.spdk:opi3"}]}}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    0,
			token:   "",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			_ = testEnv.opiSpdkServer.store.Set(testSubsystemName, &testSubsystemWithStatus)
			_ = testEnv.opiSpdkServer.store.Set(testControllerName, &testControllerWithStatus)
			_ = testEnv.opiSpdkServer.store.Set(testNamespaceName, &testNamespaceWithStatus)
			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListNvmeSubsystemsRequest{PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNvmeSubsystems(testEnv.ctx, request)

			if !utils.EqualProtoSlices(response.GetNvmeSubsystems(), tt.out) {
				t.Error("response: expected", tt.out, "received", response.GetNvmeSubsystems())
			}

			// Empty NextPageToken indicates end of results list
			if tt.size != 1 && response.GetNextPageToken() != "" {
				t.Error("Expected end of results, receieved non-empty next page token", response.GetNextPageToken())
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}
		})
	}
}

func TestFrontEnd_GetNvmeSubsystem(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.NvmeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not list NQN: %v", "nqn.2022-09.io.spdk:opi3"),
		},
		"valid request with empty SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			in: testSubsystemName,
			out: &pb.NvmeSubsystem{
				Spec:   &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi3"},
				Status: &pb.NvmeSubsystemStatus{FirmwareRevision: "TBD"},
			},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "subsys_list": [{"subnqn": "nqn.2022-09.io.spdk:opi1"},{"subnqn": "nqn.2022-09.io.spdk:opi2"},{"subnqn": "nqn.2022-09.io.spdk:opi3"}]}}`},
			errCode: codes.OK,
			errMsg:  "",
		},
		"valid request with unknown key": {
			in:      utils.ResourceIDToSubsystemName("unknown-subsystem-id"),
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", utils.ResourceIDToSubsystemName("unknown-subsystem-id")),
		},
		"malformed name": {
			in:      "-ABC-DEF",
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
		"no required field": {
			in:      "",
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: name",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			_ = testEnv.opiSpdkServer.store.Set(testSubsystemName, &testSubsystemWithStatus)
			_ = testEnv.opiSpdkServer.store.Set(testControllerName, &testControllerWithStatus)
			_ = testEnv.opiSpdkServer.store.Set(testNamespaceName, &testNamespaceWithStatus)

			request := &pb.GetNvmeSubsystemRequest{Name: tt.in}
			response, err := testEnv.client.GetNvmeSubsystem(testEnv.ctx, request)

			if !proto.Equal(response, tt.out) {
				t.Error("response: expected", tt.out, "received", response)
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}
		})
	}
}

func TestFrontEnd_StatsNvmeSubsystem(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not stats NQN: %v", "nqn.2022-09.io.spdk:opi3"),
		},
		"valid request with invalid marshal SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_get_info: %v", "json: cannot unmarshal array into Go value of type models.MrvlNvmGetSubsysInfoResult"),
		},
		"valid request with empty SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_get_info: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_get_info: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_get_info: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			in: testSubsystemName,
			out: &pb.VolumeStats{
				ReadOpsCount:  -1,
				WriteOpsCount: -1,
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"subsys_list":[{"subnqn":"nqn.2014-08.org.Nvmexpress.discovery","mn":"OCTEON NVME 0.0.1","sn":"OCTNVME0000000000002","max_namespaces":16,"min_ctrlr_id":1,"max_ctrlr_id":8,"num_ns":2,"num_total_ctrlr":2,"num_active_ctrlr":2,"ns_list":[{"ns_instance_id":1,"bdev":"bdev01","ctrlr_id_list":[{"ctrlr_id":1},{"ctrlr_id":2}]},{"ns_instance_id":1,"bdev":"bdev02","ctrlr_id_list":[{"ctrlr_id":3}]}]}]}}`},
			errCode: codes.OK,
			errMsg:  "",
		},
		"valid request with unknown key": {
			in:      "unknown-subsystem-id",
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", "unknown-subsystem-id"),
		},
		"malformed name": {
			in:      "-ABC-DEF",
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			_ = testEnv.opiSpdkServer.store.Set(testSubsystemName, &testSubsystemWithStatus)
			_ = testEnv.opiSpdkServer.store.Set(testControllerName, &testControllerWithStatus)
			_ = testEnv.opiSpdkServer.store.Set(testNamespaceName, &testNamespaceWithStatus)

			request := &pb.StatsNvmeSubsystemRequest{Name: tt.in}
			response, err := testEnv.client.StatsNvmeSubsystem(testEnv.ctx, request)

			if !proto.Equal(response.GetStats(), tt.out) {
				t.Error("response: expected", tt.out, "received", response.GetStats())
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}
		})
	}
}
