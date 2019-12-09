package aws

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ipfs/testground/pkg/config"

	"github.com/docker/docker/api/types"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
)

// ECR is a singleton object to namespace ECR operations.
var ECR = &ecrsvc{}

type ecrsvc struct{}

// newService creates a new ECR backend service stub.
func (*ecrsvc) newService(cfg config.AWSConfig) (*ecr.ECR, error) {
	config := aws.NewConfig()
	if cfg.Region != "" {
		config = config.WithRegion(cfg.Region)
	}

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		creds := credentials.NewStaticCredentials(cfg.AccessKeyID, cfg.SecretAccessKey, "")
		config = config.WithCredentials(creds)
	}
	sess, err := session.NewSession(config)
	if err != nil {
		return nil, err
	}

	svc := ecr.New(sess)
	svc.AddDebugHandlers()
	return svc, nil
}

// GetLogin returns the ECR login details for usage with the Docker API.
func (e *ecrsvc) GetAuthToken(cfg config.AWSConfig) (auth types.AuthConfig, err error) {
	svc, err := e.newService(cfg)
	if err != nil {
		return types.AuthConfig{}, err
	}
	token, err := svc.GetAuthorizationToken(nil)
	if err != nil {
		return types.AuthConfig{}, err
	} else if len(token.AuthorizationData) == 0 {
		return types.AuthConfig{}, fmt.Errorf("ecr: got zero auth tokens")
	}

	data := token.AuthorizationData[0]
	bytes, err := base64.URLEncoding.DecodeString(*data.AuthorizationToken)
	if err != nil {
		return types.AuthConfig{}, fmt.Errorf("ecr: failed to decode base64: %w", err)
	}

	splt := strings.Split(string(bytes), ":")
	if len(splt) != 2 {
		return types.AuthConfig{}, fmt.Errorf("ecr: unexpected format for auth token: %v", splt)
	}

	var (
		user     = splt[0]
		pwd      = splt[1]
		endpoint = *data.ProxyEndpoint
	)

	auth = types.AuthConfig{
		Username:      user,
		Password:      pwd,
		ServerAddress: endpoint,
	}

	return auth, nil
}

func (e *ecrsvc) EncodeAuthToken(token types.AuthConfig) string {
	// Marshal the token and encode it into base64. That's how registries expect
	// the Authentication token to be passed.
	// See https://forums.docker.com/t/how-to-create-registryauth-for-private-registry-login-credentials/29235
	t, _ := json.Marshal(token)
	return base64.URLEncoding.EncodeToString(t)
}

func (e *ecrsvc) EnsureRepository(cfg config.AWSConfig, name string) (uri string, err error) {
	svc, err := e.newService(cfg)
	if err != nil {
		return "", err
	}

	// Check if the repository exists.
	d, err := svc.DescribeRepositories(&ecr.DescribeRepositoriesInput{RepositoryNames: []*string{&name}})

	// ErrCodeRepositoryNotFoundException is the code we expect if the repository doesn't exist.
	// If we get a different error, abort.
	if err != nil && err.(awserr.Error).Code() != ecr.ErrCodeRepositoryNotFoundException {
		return "", err
	}
	if len(d.Repositories) > 0 {
		// The repository exists. Pick the first match.
		return *d.Repositories[0].RepositoryUri, nil
	}

	c, err := svc.CreateRepository(&ecr.CreateRepositoryInput{RepositoryName: &name})
	if err != nil {
		return "", err
	} else if c.Repository == nil {
		return "", fmt.Errorf("ecr: newly created repository was nil")
	}

	return *c.Repository.RepositoryUri, nil
}
