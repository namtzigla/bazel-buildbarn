package builder

import (
	"io"
	"os"

	"github.com/EdSchouten/bazel-buildbarn/pkg/blobstore"

	remoteexecution "google.golang.org/genproto/googleapis/devtools/remoteexecution/v1test"
)

type blobAccessInputFileExposer struct {
	contentAddressableStorage blobstore.BlobAccess
}

func NewBlobAccessInputFileExposer(contentAddressableStorage blobstore.BlobAccess) InputFileExposer {
	return &blobAccessInputFileExposer{
		contentAddressableStorage: contentAddressableStorage,
	}
}

func (fe *blobAccessInputFileExposer) Expose(instance string, digest *remoteexecution.Digest, outputPath string, isExecutable bool) error {
	var mode os.FileMode = 0444
	if isExecutable {
		mode = 0555
	}
	f, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer f.Close()

	// TODO(edsch): Translate NOT_FOUND to INVALID_PRECONDITION?
	r, err := fe.contentAddressableStorage.Get(instance, digest)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	return err
}