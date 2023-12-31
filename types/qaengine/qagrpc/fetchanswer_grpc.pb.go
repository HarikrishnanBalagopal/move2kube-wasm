/*
 *  Copyright IBM Corporation 2020, 2021, 2022
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.19.1
// source: fetchanswer.proto

package qagrpc

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// QAEngineClient is the client API for QAEngine service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type QAEngineClient interface {
	FetchAnswer(ctx context.Context, in *Problem, opts ...grpc.CallOption) (*Answer, error)
}

type qAEngineClient struct {
	cc grpc.ClientConnInterface
}

func NewQAEngineClient(cc grpc.ClientConnInterface) QAEngineClient {
	return &qAEngineClient{cc}
}

func (c *qAEngineClient) FetchAnswer(ctx context.Context, in *Problem, opts ...grpc.CallOption) (*Answer, error) {
	out := new(Answer)
	err := c.cc.Invoke(ctx, "/qagrpc.QAEngine/FetchAnswer", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// QAEngineServer is the server API for QAEngine service.
// All implementations must embed UnimplementedQAEngineServer
// for forward compatibility
type QAEngineServer interface {
	FetchAnswer(context.Context, *Problem) (*Answer, error)
	mustEmbedUnimplementedQAEngineServer()
}

// UnimplementedQAEngineServer must be embedded to have forward compatible implementations.
type UnimplementedQAEngineServer struct {
}

func (UnimplementedQAEngineServer) FetchAnswer(context.Context, *Problem) (*Answer, error) {
	return nil, status.Errorf(codes.Unimplemented, "method FetchAnswer not implemented")
}
func (UnimplementedQAEngineServer) mustEmbedUnimplementedQAEngineServer() {}

// UnsafeQAEngineServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to QAEngineServer will
// result in compilation errors.
type UnsafeQAEngineServer interface {
	mustEmbedUnimplementedQAEngineServer()
}

func RegisterQAEngineServer(s grpc.ServiceRegistrar, srv QAEngineServer) {
	s.RegisterService(&QAEngine_ServiceDesc, srv)
}

func _QAEngine_FetchAnswer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Problem)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QAEngineServer).FetchAnswer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/qagrpc.QAEngine/FetchAnswer",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QAEngineServer).FetchAnswer(ctx, req.(*Problem))
	}
	return interceptor(ctx, in, info, handler)
}

// QAEngine_ServiceDesc is the grpc.ServiceDesc for QAEngine service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var QAEngine_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "qagrpc.QAEngine",
	HandlerType: (*QAEngineServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "FetchAnswer",
			Handler:    _QAEngine_FetchAnswer_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "fetchanswer.proto",
}
