package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strings"

	"github.com/EdSchouten/bazel-buildbarn/pkg/ac"
	"github.com/EdSchouten/bazel-buildbarn/pkg/blobstore"
	"github.com/EdSchouten/bazel-buildbarn/pkg/builder"
	"github.com/EdSchouten/bazel-buildbarn/pkg/cas"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc"
)

type stringList []string

func (i *stringList) String() string {
	return "my string representation"
}

func (i *stringList) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	var schedulersList stringList
	var (
		actionCacheAllowUpdates = flag.Bool("ac-allow-updates", false, "Allow clients to write into the action cache")
		blobstoreConfig         = flag.String("blobstore-config", "/config/blobstore.conf", "Configuration for blob storage")
	)
	flag.Var(&schedulersList, "scheduler", "Backend capable of executing build actions. Example: debian8|hostname-of-debian8-scheduler:8981")
	flag.Parse()

	// Web server for metrics and profiling.
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Fatal(http.ListenAndServe(":80", nil))
	}()

	// Storage access.
	contentAddressableStorageBlobAccess, actionCacheBlobAccess, err := blobstore.CreateBlobAccessObjectsFromConfig(*blobstoreConfig)
	if err != nil {
		log.Fatal("Failed to create blob access: ", err)
	}
	actionCache := ac.NewBlobAccessActionCache(actionCacheBlobAccess)

	// Backends capable of compiling.
	schedulers := map[string]remoteexecution.ExecutionServer{}
	for _, schedulerEntry := range schedulersList {
		components := strings.SplitN(schedulerEntry, "|", 2)
		if len(components) != 2 {
			log.Fatal("Invalid scheduler entry: ", schedulerEntry)
		}
		scheduler, err := grpc.Dial(
			components[1],
			grpc.WithInsecure(),
			grpc.WithUnaryInterceptor(grpc_prometheus.UnaryClientInterceptor),
			grpc.WithStreamInterceptor(grpc_prometheus.StreamClientInterceptor))
		if err != nil {
			log.Fatal("Failed to create scheduler RPC client: ", err)
		}
		schedulers[components[0]] = builder.NewForwardingBuildQueue(scheduler)
	}
	buildQueue := builder.NewDemultiplexingBuildQueue(schedulers)

	// RPC server.
	s := grpc.NewServer(
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
	)
	remoteexecution.RegisterActionCacheServer(s, ac.NewActionCacheServer(actionCache, *actionCacheAllowUpdates))
	remoteexecution.RegisterContentAddressableStorageServer(s, cas.NewContentAddressableStorageServer(contentAddressableStorageBlobAccess))
	bytestream.RegisterByteStreamServer(s, blobstore.NewByteStreamServer(contentAddressableStorageBlobAccess))
	remoteexecution.RegisterExecutionServer(s, buildQueue)
	grpc_prometheus.EnableHandlingTimeHistogram()
	grpc_prometheus.Register(s)

	sock, err := net.Listen("tcp", ":8980")
	if err != nil {
		log.Fatal("Failed to create listening socket: ", err)
	}
	if err := s.Serve(sock); err != nil {
		log.Fatal("Failed to serve RPC server: ", err)
	}
}
