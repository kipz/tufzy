package client

const (
	// TUF annotation key for filename in OCI layers
	TUFFilenameAnnotation = "tuf.io/filename"

	// TUF media types
	TUFMetadataMediaType = "application/vnd.tuf.metadata+json"
	TUFTargetMediaType   = "application/vnd.tuf.target"

	// OCI URL scheme
	OCIScheme = "oci://"
)
