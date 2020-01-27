package cmd

import (
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/ec2metadata"
	"github.com/aws/aws-sdk-go-v2/aws/external"
)

// AWSFlags represents a set of flags for connecting to AWS.
type AWSFlags struct {
	// Name of AWS region to use.
	Region string

	// Name of a shared AWS credentials profile to use.
	Profile string

	// Max number of retries to attempt on connection error.
	MaxRetries int
}

// NewAWSFlags returns a new BaseFlags.
func NewAWSFlags(app Flagger, maxRetries int) *AWSFlags {
	var f AWSFlags

	app.Flag("aws.region", "Name of AWS region to use.").
		PlaceHolder("REGION_NAME").
		StringVar(&f.Region)

	app.Flag("aws.profile", "Name of AWS credentials profile to use.").
		PlaceHolder("PROFILE_NAME").
		StringVar(&f.Profile)

	app.Flag("aws.max-retries", "Max number of retries to attempt on connection failure.").
		Hidden().
		Default(strconv.Itoa(maxRetries)).
		IntVar(&f.MaxRetries)

	return &f
}

// AWSConfig returns a aws.Config configure based on the default AWS
// config and these flags.
func (f *AWSFlags) AWSConfig(opts ...external.Config) aws.Config {
	if f.Region != "" {
		opts = append(opts, external.WithRegion(f.Region))
	}

	if f.Profile != "" {
		opts = append(opts, external.WithSharedConfigProfile(f.Profile))
	}

	cfg, err := external.LoadDefaultAWSConfig(opts...)
	if err != nil {
		panic("unable to load AWS SDK default config, " + err.Error())
	}

	if cfg.Region == "" {
		// Try setting region from EC2 metadata.
		metaClient := ec2metadata.New(cfg)
		if region, err := metaClient.Region(); err == nil {
			cfg.Region = region
		}
	}

	cfg.Retryer = aws.DefaultRetryer{
		NumMaxRetries: f.MaxRetries,
	}

	return cfg
}
