package ac

import (
	"bytes"
	"context"
	"io/ioutil"

	"github.com/EdSchouten/bazel-buildbarn/pkg/blobstore"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/golang/protobuf/proto"
)

type blobAccessActionCache struct {
	blobAccess blobstore.BlobAccess
}

// NewBlobAccessActionCache creates an ActionCache object that reads and
// writes action cache entries from a BlobAccess based store.
func NewBlobAccessActionCache(blobAccess blobstore.BlobAccess) ActionCache {
	return &blobAccessActionCache{
		blobAccess: blobAccess,
	}
}

func (ac *blobAccessActionCache) GetActionResult(ctx context.Context, instance string, digest *remoteexecution.Digest) (*remoteexecution.ActionResult, error) {
	r := ac.blobAccess.Get(ctx, instance, digest)
	data, err := ioutil.ReadAll(r)
	r.Close()
	if err != nil {
		return nil, err
	}
	var actionResult remoteexecution.ActionResult
	if err := proto.Unmarshal(data, &actionResult); err != nil {
		return nil, err
	}
	return &actionResult, nil
}

func (ac *blobAccessActionCache) PutActionResult(ctx context.Context, instance string, digest *remoteexecution.Digest, result *remoteexecution.ActionResult) error {
	data, err := proto.Marshal(result)
	if err != nil {
		return err
	}
	return ac.blobAccess.Put(ctx, instance, digest, int64(len(data)), ioutil.NopCloser(bytes.NewBuffer(data)))
}
