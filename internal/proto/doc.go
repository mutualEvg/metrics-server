// Package proto provides Protocol Buffer definitions and generated code for gRPC communication.
//
//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative metrics.proto
package proto
