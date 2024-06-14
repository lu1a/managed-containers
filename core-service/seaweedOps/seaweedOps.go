package seaweedOps

// Create a bucket as "me", the admin, because seaweed doesn't have separate sub-users or IAM
func CreateBucket() {
	// TODO: makey bucket with the project's name and ID
}

// We want to replicate the s3 API provided by seaweed s3 without rewriting and proxying every single indivudal file CRUD operation
func ProxyOperationToBucket() {
	// TODO: In general just a check here that the user owns the bucket they're trying to mess with
}
