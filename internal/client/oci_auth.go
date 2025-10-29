package client

import (
	"io"

	ecr "github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// MultiKeychainOption returns a remote.Option that uses the multi-keychain for authentication.
func MultiKeychainOption() remote.Option {
	return remote.WithAuthFromKeychain(MultiKeychainAll())
}

// MultiKeychainAll creates a keychain that tries Docker config, Google Cloud, and AWS ECR authentication.
func MultiKeychainAll() authn.Keychain {
	// Create a multi-keychain that will use the default Docker, Google, or ECR keychain
	return authn.NewMultiKeychain(
		authn.DefaultKeychain,
		google.Keychain,
		authn.NewKeychainFromHelper(ecr.NewECRHelper(ecr.WithLogger(io.Discard))),
	)
}
