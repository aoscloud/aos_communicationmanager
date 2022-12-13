// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.6.1
// source: servicemanager/v3/servicemanager.proto

package servicemanager

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

// SMServiceClient is the client API for SMService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SMServiceClient interface {
	RegisterSM(ctx context.Context, opts ...grpc.CallOption) (SMService_RegisterSMClient, error)
}

type sMServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewSMServiceClient(cc grpc.ClientConnInterface) SMServiceClient {
	return &sMServiceClient{cc}
}

func (c *sMServiceClient) RegisterSM(ctx context.Context, opts ...grpc.CallOption) (SMService_RegisterSMClient, error) {
	stream, err := c.cc.NewStream(ctx, &SMService_ServiceDesc.Streams[0], "/servicemanager.v3.SMService/RegisterSM", opts...)
	if err != nil {
		return nil, err
	}
	x := &sMServiceRegisterSMClient{stream}
	return x, nil
}

type SMService_RegisterSMClient interface {
	Send(*SMOutgoingMessages) error
	Recv() (*SMIncomingMessages, error)
	grpc.ClientStream
}

type sMServiceRegisterSMClient struct {
	grpc.ClientStream
}

func (x *sMServiceRegisterSMClient) Send(m *SMOutgoingMessages) error {
	return x.ClientStream.SendMsg(m)
}

func (x *sMServiceRegisterSMClient) Recv() (*SMIncomingMessages, error) {
	m := new(SMIncomingMessages)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// SMServiceServer is the server API for SMService service.
// All implementations must embed UnimplementedSMServiceServer
// for forward compatibility
type SMServiceServer interface {
	RegisterSM(SMService_RegisterSMServer) error
	mustEmbedUnimplementedSMServiceServer()
}

// UnimplementedSMServiceServer must be embedded to have forward compatible implementations.
type UnimplementedSMServiceServer struct {
}

func (UnimplementedSMServiceServer) RegisterSM(SMService_RegisterSMServer) error {
	return status.Errorf(codes.Unimplemented, "method RegisterSM not implemented")
}
func (UnimplementedSMServiceServer) mustEmbedUnimplementedSMServiceServer() {}

// UnsafeSMServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SMServiceServer will
// result in compilation errors.
type UnsafeSMServiceServer interface {
	mustEmbedUnimplementedSMServiceServer()
}

func RegisterSMServiceServer(s grpc.ServiceRegistrar, srv SMServiceServer) {
	s.RegisterService(&SMService_ServiceDesc, srv)
}

func _SMService_RegisterSM_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(SMServiceServer).RegisterSM(&sMServiceRegisterSMServer{stream})
}

type SMService_RegisterSMServer interface {
	Send(*SMIncomingMessages) error
	Recv() (*SMOutgoingMessages, error)
	grpc.ServerStream
}

type sMServiceRegisterSMServer struct {
	grpc.ServerStream
}

func (x *sMServiceRegisterSMServer) Send(m *SMIncomingMessages) error {
	return x.ServerStream.SendMsg(m)
}

func (x *sMServiceRegisterSMServer) Recv() (*SMOutgoingMessages, error) {
	m := new(SMOutgoingMessages)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// SMService_ServiceDesc is the grpc.ServiceDesc for SMService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SMService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "servicemanager.v3.SMService",
	HandlerType: (*SMServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "RegisterSM",
			Handler:       _SMService_RegisterSM_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "servicemanager/v3/servicemanager.proto",
}
