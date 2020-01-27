package events

// AZSubnet is nested in the details of several
// AutoScaling events.
//
// Example:
//
//   "Details": {
//       "Availability Zone": "us-west-2b",
//       "Subnet ID": "subnet-12345678"
//   }
//
type AZSubnet struct {
	// Example: "us-west-2b"
	AvailabilityZone string `json:"Availability Zone"`

	// Example: "subnet-12345678
	SubnetID string `json:"Subnet ID"`
}
