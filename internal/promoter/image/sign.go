/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package imagepromoter

import (
	"context"
	"fmt"

	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	credentialspb "google.golang.org/genproto/googleapis/iam/credentials/v1"

	"sigs.k8s.io/release-sdk/sign"

	reg "sigs.k8s.io/promo-tools/v3/internal/legacy/dockerregistry"
	options "sigs.k8s.io/promo-tools/v3/promoter/image/options"
)

const (
	TestSigningAccount = ""
)

// ValidateStagingSignatures checks if edges (images) have a signature
// applied during its staging run. If they do it verifies them and
// returns an error if they are not valid.
func (di *DefaultPromoterImplementation) ValidateStagingSignatures(
	edges map[reg.PromotionEdge]interface{},
) error {
	signer := sign.New(sign.Default())

	for edge := range edges {
		imageRef := fmt.Sprintf(
			"%s/%s@%s",
			edge.SrcRegistry.Name,
			edge.SrcImageTag.Name,
			edge.Digest,
		)
		logrus.Infof("Verifying signatures of image %s", imageRef)

		// If image is not signed, skip
		isSigned, err := signer.IsImageSigned(imageRef)
		if err != nil {
			return errors.Wrapf(err, "checking if %s is signed", imageRef)
		}

		if !isSigned {
			logrus.Infof("No signatures found for ref %s, not checking", imageRef)
			continue
		}

		// Check the staged image signatures
		if _, err := signer.VerifyImage(imageRef); err != nil {
			return errors.Wrapf(
				err, "verifying signatures of image %s", imageRef,
			)
		}
		logrus.Infof("Signatures for ref %s verfified", imageRef)
	}
	return nil
}

// SignImages signs the promoted images and stores their signatures in
// the registry
func (di *DefaultPromoterImplementation) SignImages(
	opts *options.Options, sc *reg.SyncContext, edges map[reg.PromotionEdge]interface{},
) error {
	signer := sign.New(sign.Default())
	for edge := range edges {
		imageRef := fmt.Sprintf(
			"%s/%s@%s",
			edge.DstRegistry.Name,
			edge.DstImageTag.Name,
			edge.Digest,
		)
		if _, err := signer.SignImage(imageRef); err != nil {
			return errors.Wrapf(err, "signing image %s", imageRef)
		}
		logrus.Infof("Signing image %s", imageRef)
	}
	return nil
}

// WriteSBOMs writes SBOMs to each of the newly promoted images and stores
// them along the signatures in the registry
func (di *DefaultPromoterImplementation) WriteSBOMs(
	opts *options.Options, sc *reg.SyncContext, edges map[reg.PromotionEdge]interface{},
) error {
	return nil
}

// GetIdentityToken returns an identity token for the selected service account
// in order for this function to work, an account has to be already logged. This
// can be achieved using the
func (di *DefaultPromoterImplementation) GetIdentityToken(
	opts *options.Options, serviceAccount string,
) (tok string, err error) {
	ctx := context.Background()
	c, err := credentials.NewIamCredentialsClient(ctx)
	if err != nil {
		return tok, errors.Wrap(err, "creating credentials token")
	}
	defer c.Close()
	req := &credentialspb.GenerateIdTokenRequest{
		Name:         fmt.Sprintf("projects/-/serviceAccounts/%s", serviceAccount),
		Audience:     "sigstore",
		IncludeEmail: true,
	}

	resp, err := c.GenerateIdToken(ctx, req)
	if err != nil {
		return tok, errors.Wrap(err, "getting error account")
	}

	return resp.Token, nil
}
