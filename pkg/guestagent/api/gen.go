//go:generate protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative guestservice.proto --descriptor_set_out=guestservice.pb.desc

package api
