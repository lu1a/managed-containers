package kubeOps

type addedResourceToRollBack struct {
	zone         string // ie. client.Name
	resourceType string
	namespace    string
	name         string
}

type LogsForZone struct {
	Zone string   `json:"zone"`
	Logs []string `json:"logs"`
}
