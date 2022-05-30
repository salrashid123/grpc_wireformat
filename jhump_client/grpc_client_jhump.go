package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	ejson "encoding/json"
	"flag"
	"fmt"
	"io/ioutil"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

/*
example taken from:   https://github.com/jhump/protoreflect/issues/332
*/

const (
	serviceName = "echo.EchoServer"
	methodName  = "SayHello"
)

var (
	cacert          = flag.String("cacert", "../grpc_services/certs/tls-ca-chain.pem", "CACert for server")
	addr            = flag.String("addr", "localhost:50051", "host:port of grpc server")
	protoImportPath = flag.String("protoImportPath", "../grpc_services/src/echo/", "path to .proto file")
	serverName      = flag.String("servername", "localhost", "SNI for server")
)

func main() {

	flag.Parse()

	parser := protoparse.Parser{}
	parser.ImportPaths = []string{
		*protoImportPath,
	}

	descs, err := parser.ParseFiles("echo.proto")
	if err != nil {
		fmt.Printf("did not load ca: %v\n", err)
		return
	}

	var descriptor desc.FileDescriptor
	for _, desc := range descs {
		for _, service := range desc.GetServices() {
			fmt.Printf("> service %s\n", service.GetFullyQualifiedName())
			for _, method := range service.GetMethods() {
				fmt.Printf("  * method %s (%s) %s\n",
					method.GetFullyQualifiedName(),
					method.GetInputType().GetFullyQualifiedName(),
					method.GetOutputType().GetFullyQualifiedName(),
				)
			}

			if service.GetFullyQualifiedName() == serviceName {
				descriptor = *desc.GetFile()
				break
			}

		}
		for _, msg := range desc.GetMessageTypes() {
			fmt.Printf("- message %s\n", msg.GetFullyQualifiedName())
		}
	}

	messageDesc := descriptor.FindMessage("echo.EchoRequest")
	if messageDesc == nil {
		panic(fmt.Errorf("Can't find message"))
	}
	message := dynamic.NewMessage(messageDesc)

	message.SetFieldByName("first_name", "sal")
	message.SetFieldByName("last_name", "amander")

	fmt.Printf("Looking for serviceName %s methodName %s\n", serviceName, methodName)

	serviceDesc := descriptor.FindService(serviceName)
	if serviceDesc == nil {
		panic(fmt.Errorf("Can't find service %s", serviceName))
	}

	methodDesc := serviceDesc.FindMethodByName(methodName)
	if methodDesc == nil {
		panic(fmt.Errorf("Can't find method %s in service %s", methodName, serviceName))
	}

	// Create grpc call

	caCert, err := ioutil.ReadFile(*cacert)
	if err != nil {
		panic(fmt.Errorf("did not load ca: %v\n", err))
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := tls.Config{
		ServerName: *serverName,
		RootCAs:    caCertPool,
	}

	creds := credentials.NewTLS(&tlsConfig)

	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	grpcClient := grpcdynamic.NewStub(conn)

	// SENDING MESSAGE

	response, err := grpcClient.InvokeRpc(context.TODO(), methodDesc, message)
	if err != nil {
		panic(err)
	}

	json, err := response.(*dynamic.Message).MarshalJSON()
	if err != nil {
		panic(err)
	}

	var prettyJSON bytes.Buffer
	error := ejson.Indent(&prettyJSON, json, "", "\t")
	if error != nil {
		fmt.Printf("JSON parse error: %v\n", error)
		return
	}

	fmt.Printf("Response: %s\n", string(prettyJSON.Bytes()))

}
