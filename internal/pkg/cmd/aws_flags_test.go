package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert" // Test assertions e.g. equality.

	kingpin "gopkg.in/alecthomas/kingpin.v2" // Command line flag parsing.
)

func TestNewAWSFlags(t *testing.T) {
	app := kingpin.New("testapp", "usage")
	f := NewAWSFlags(app, 5)
	_, err := app.Parse([]string{
		"--aws.region", "us-east-2",
		"--aws.profile", "foobar",
		"--aws.max-retries", "1",
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, f.MaxRetries)
	assert.Equal(t, "us-east-2", f.Region)
	assert.Equal(t, "foobar", f.Profile)
}

func TestAWSFlags_AWSConfig(t *testing.T) {
	f := &AWSFlags{
		Region:     "us-east-2",
		Profile:    "foobar",
		MaxRetries: 5,
	}
	cfg := f.AWSConfig()
	assert.Equal(t, "us-east-2", cfg.Region)
	assert.Equal(t, 5, cfg.Retryer.MaxRetries())
}
