package types

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
	"k8s.io/client-go/kubernetes"
)

type Config struct {
	ListenURL       string
	ShutdownTimeout time.Duration

	GitHubClientID     string
	GitHubClientSecret string

	AdminDBConnectionURL string

	KubeClients []ContainerZone

	UserDBConnections []UserDB
}

type UserDB struct {
	Zone          string `json:"zone"`
	ID            string `json:"id"`
	ConnectionURL string `json:"connection_url"`
}

func (userDB UserDB) DefaultParentEnvironmentURL() string {
	return userDB.ConnectionURL + "/postgres"
}

func (userDB UserDB) ConnWithSuffix(suffix string) string {
	return userDB.ConnectionURL + "/" + suffix
}

type UserDBConnectionsRaw struct {
	Zones []UserDB `json:"zones"`
}

func (c *Config) GetZonesFromUserDBConnections() (zones []string) {
	for _, userDBConn := range c.UserDBConnections {
		zones = append(zones, userDBConn.Zone)
	}
	return zones
}

type KubeClientsRaw struct {
	Clients []ContainerZone `json:"clients"`
}

type ContainerZone struct {
	Name             string `json:"name" db:"name"`
	DefaultRoutingIP string `json:"default_routing_ip" db:"default_routing_ip"`
	CPUMilliCores    int    `json:"cpu_millicores" db:"cpu_millicores"`
	MemoryMB         int    `json:"memory_mb" db:"memory_mb"`

	ClientSet *kubernetes.Clientset
}

func GetZonesFromContainerZones(clients []ContainerZone) (zones []string) {
	for _, client := range clients {
		zones = append(zones, client.Name)
	}
	return zones
}

type Account struct {
	AccountID      int        `json:"account_id" db:"account_id"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	DeletedAt      *time.Time `json:"deleted_at" db:"deleted_at"`
	Name           string     `json:"name" db:"name"`
	Username       string     `json:"username" db:"username"`
	Email          *string    `json:"email" db:"email"`
	Location       *string    `json:"location" db:"location"`
	AvatarURL      *string    `json:"avatar_url" db:"avatar_url"`
	PasswordHash   *string    `json:"-" db:"password_hash"`
	PasswordSalt   *string    `json:"-" db:"password_salt"`
	SuspendedAt    *time.Time `json:"suspended_at" db:"suspended_at"`
	BecameMemberAt *time.Time `json:"became_member_at" db:"became_member_at"`
}

type GitHubAccountProfile struct {
	ProfileID         int       `json:"profile_id" db:"profile_id"`
	AccountID         int       `json:"account_id" db:"account_id"`
	UserProfileID     int       `json:"id" db:"user_profile_id"` // the real ID from GitHub's side
	AvatarURL         string    `json:"avatar_url" db:"avatar_url"`
	Bio               *string   `json:"bio" db:"bio"`
	Blog              string    `json:"blog" db:"blog"`
	Company           string    `json:"company" db:"company"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	Email             *string   `json:"email" db:"email"`
	EventsURL         string    `json:"events_url" db:"events_url"`
	Followers         int       `json:"followers" db:"followers"`
	FollowersURL      string    `json:"followers_url" db:"followers_url"`
	Following         int       `json:"following" db:"following"`
	FollowingURL      string    `json:"following_url" db:"following_url"`
	GistsURL          string    `json:"gists_url" db:"gists_url"`
	GravatarID        string    `json:"gravatar_id" db:"gravatar_id"`
	Hireable          *bool     `json:"hireable" db:"hireable"`
	HTMLURL           string    `json:"html_url" db:"html_url"`
	Location          string    `json:"location" db:"location"`
	Login             string    `json:"login" db:"login"`
	Name              string    `json:"name" db:"name"`
	NodeID            string    `json:"node_id" db:"node_id"`
	OrganizationsURL  string    `json:"organizations_url" db:"organizations_url"`
	PublicGists       int       `json:"public_gists" db:"public_gists"`
	PublicRepos       int       `json:"public_repos" db:"public_repos"`
	ReceivedEventsURL string    `json:"received_events_url" db:"received_events_url"`
	ReposURL          string    `json:"repos_url" db:"repos_url"`
	SiteAdmin         bool      `json:"site_admin" db:"site_admin"`
	StarredURL        string    `json:"starred_url" db:"starred_url"`
	SubscriptionsURL  string    `json:"subscriptions_url" db:"subscriptions_url"`
	TwitterUsername   *string   `json:"twitter_username" db:"twitter_username"`
	UserType          string    `json:"user_type" db:"user_type"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
	URL               string    `json:"url" db:"url"`
}

type Session struct {
	SessionID    int    `json:"session_id" db:"session_id"`
	SessionToken string `json:"token" db:"token"`
	AccountID    int    `json:"account_id" db:"account_id"`
}

type APIToken struct {
	APITokenID int    `json:"api_token_id" db:"api_token_id"`
	APIToken   string `json:"token" db:"token"`
	AccountID  int    `json:"account_id" db:"account_id"`
}

type Project struct {
	ProjectID   int        `json:"project_id" db:"project_id"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	DeletedAt   *time.Time `json:"deleted_at" db:"deleted_at"`
	Name        string     `json:"name" db:"name"`
	Description string     `json:"description" db:"description"`

	SharedUsernames []string `json:"shared_users"`
}

func (p *Project) NamespaceName() string {
	return fmt.Sprintf("namespace-%s-%v", p.Name, p.ProjectID)
}

func (p Project) UserDBClaimName() string {
	return fmt.Sprintf("db_%s_%v", strings.ReplaceAll(strings.ToLower(p.Name), "-", "_"), p.ProjectID)
}

func (p Project) UserDBClaimSchemaName() string {
	return fmt.Sprintf("%s_schema", p.UserDBClaimName())
}

func (p Project) UserDBClaimRWUsername() string {
	return fmt.Sprintf("%s_user_rw", p.UserDBClaimName())
}

func (p Project) UserDBClaimROUsername() string {
	return fmt.Sprintf("%s_user_ro", p.UserDBClaimName())
}

type AccountProjectMap struct {
	ProjectID int `json:"project_id" db:"project_id"`
	AccountID int `json:"account_id" db:"account_id"`
}

type ContainerClaim struct {
	ContainerClaimID int        `json:"container_claim_id" db:"container_claim_id"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	DeletedAt        *time.Time `json:"deleted_at" db:"deleted_at"`
	Name             string     `json:"name" db:"name"`
	ImageRef         string     `json:"image_ref" db:"image_ref"`
	ImageTag         string     `json:"image_tag" db:"image_tag"`

	Command pq.StringArray `json:"command" db:"command"` // optional: such as the command: ["perl",  "-Mbignum=bpi", "-wle", "print bpi(2000)"]

	NodeIP      *string       `json:"node_ip" db:"node_ip"`           // the public IP address of the node that this container actually sits on
	Ports       pq.Int64Array `json:"ports" db:"ports"`               // the public ports I'm going to route to this container
	TargetPorts pq.Int64Array `json:"target_ports" db:"target_ports"` // the ports the user actually wants to expose

	CPUMilliCores int `json:"cpu_millicores" db:"cpu_millicores"`
	MemoryMB      int `json:"memory_mb" db:"memory_mb"`

	Status      string         `json:"status" db:"status"`     // inactive | active | deactivating | activating | error
	RunType     string         `json:"run_type" db:"run_type"` // permanent | once | schedule
	Zones       pq.StringArray `json:"zones" db:"zones"`
	EnvVarNames pq.StringArray `json:"env_var_names" db:"env_var_names"`

	CreatedByAccountID int `json:"created_by_account_id" db:"created_by_account_id"`
	ProjectID          int `json:"project_id" db:"project_id"`

	EnvVars         []EnvVar         `json:"-"`
	ImagePullSecret *ImagePullSecret `json:"-"`
}

// For permanently running containers
func (c *ContainerClaim) DeploymentName() string {
	return fmt.Sprintf("deployment-%s-%v", c.Name, c.ContainerClaimID)
}

// For run-once containers
func (c *ContainerClaim) JobName() string {
	return fmt.Sprintf("job-%s-%v", c.Name, c.ContainerClaimID)
}

func (c *ContainerClaim) IsRunOnce() bool {
	return c.RunType == "once"
}

func (c *ContainerClaim) ServiceName(targetPort int64) string {
	return fmt.Sprintf("service-%s-%v-%v", c.Name, c.ContainerClaimID, targetPort)
}

func (c *ContainerClaim) InternalHostName(port string) string {
	return fmt.Sprintf("host-%v-%s-%s", c.ProjectID, c.Name, port)
}

func (c *ContainerClaim) WholeImageWithTag() string {
	return fmt.Sprintf("%s:%s", c.ImageRef, c.ImageTag)
}

func (c *ContainerClaim) CPUMilliCoresAsResourceListStr() string {
	return fmt.Sprintf("%vm", c.CPUMilliCores)
}

func (c *ContainerClaim) MemoryMBAsResourceListStr() string {
	return fmt.Sprintf("%vMi", c.MemoryMB)
}

func (c *ContainerClaim) IPWithPortsDisplayStr() string {
	portMappingDisplay := []string{}
	for i, targetPort := range c.TargetPorts {
		actualPort := c.Ports[i]
		ipOrEmpty := ""
		if c.NodeIP == nil {
			ipOrEmpty = ""
		} else {
			ipOrEmpty = *c.NodeIP
		}
		portMappingDisplay = append(portMappingDisplay, fmt.Sprintf("%v -> %s:%v", targetPort, ipOrEmpty, actualPort))
	}
	return strings.Join(portMappingDisplay, ", ")
}

type EnvVar struct {
	Name  string `json:"name" db:"name"`
	Value string `json:"value" db:"value"`
}

func (c *ContainerClaim) GetEnvNamesFromVars() (names []string) {
	for _, envVar := range c.EnvVars {
		names = append(names, envVar.Name)
	}
	return names
}

func (c *ContainerClaim) EnvVarSpecName(envVarName string) string {
	return fmt.Sprintf("secret-%s-%s", c.Name, strings.ReplaceAll(strings.ToLower(envVarName), "_", "-"))
}

// for private docker registry auth
type ImagePullSecret struct {
	URL string

	Email    string
	Username string
	Password string

	// might have a token which isn't already a base64 of username:password
	Token string
}

func (c *ContainerClaim) ParseContainerFieldsFromHTTPFormZoneProject(r *http.Request, zoneNames []string, ProjectID int) (ContainerClaim, error) {
	for i := 0; i < len(r.Form["env-var-name[]"]); i++ {
		envVar := EnvVar{
			Name:  r.Form["env-var-name[]"][i],
			Value: r.Form["env-var-value[]"][i],
		}
		if envVar.Name != "" {
			c.EnvVars = append(c.EnvVars, envVar)
		}
	}

	for i := 0; i < len(r.Form["command[]"]); i++ {
		newCommandSubSection := r.Form["command[]"][i]
		if newCommandSubSection != "" {
			c.Command = append(c.Command, newCommandSubSection)
		}
	}

	for i := 0; i < len(r.Form["port[]"]); i++ {
		newPortSubSection := r.Form["port[]"][i]
		if newPortSubSection != "" {
			newPortSubSectionInt, err := strconv.Atoi(newPortSubSection)
			if err != nil {
				return *c, err
			}

			c.TargetPorts = append(c.TargetPorts, int64(newPortSubSectionInt))
		}
	}

	// pre-filling Ports with something now, but those public ports are gonna be overwritten with random ones later
	c.Ports = c.TargetPorts

	for i := 0; i < len(r.Form["zone[]"]); i++ {
		zone := r.Form["zone[]"][i]
		if zone != "" {
			c.Zones = append(c.Zones, zone)
		}
	}
	if len(c.Zones) == 0 {
		c.Zones = zoneNames
	}

	c.ProjectID = ProjectID
	c.ImagePullSecret = &ImagePullSecret{
		URL: r.FormValue("image-pull-secret-url"), // btw if the URL is "" there's no point in the pull secret, so that's what we'll use later to either add it to k8s or not

		Email:    r.FormValue("image-pull-secret-email"),
		Username: r.FormValue("image-pull-secret-username"),
		Password: r.FormValue("image-pull-secret-password"),

		Token: r.FormValue("image-pull-secret-token"),
	}

	c.Name = r.FormValue("name")
	c.ImageRef = r.FormValue("image-ref")
	c.ImageTag = r.FormValue("image-tag")
	c.RunType = r.FormValue("run-type")
	cpuMilliCores, err := strconv.Atoi(r.FormValue("cpu-millicores"))
	if err != nil {
		return *c, err
	}
	c.CPUMilliCores = cpuMilliCores
	memoryMB, err := strconv.Atoi(r.FormValue("memory-mb"))
	if err != nil {
		return *c, err
	}
	c.MemoryMB = memoryMB
	c.EnvVarNames = c.GetEnvNamesFromVars()

	// add the image pull secret name as an "env var" name bc it kinda is, so that upon cleanup later, it'll also be deleted
	if c.ImagePullSecret.URL != "" {
		c.EnvVarNames = append(c.EnvVarNames, "image-pull-secret")
	}
	if c.ImageTag == "" {
		c.ImageTag = "latest"
	}
	if c.RunType == "" {
		c.RunType = "permanent"
	}

	return *c, nil
}

type UserDBClaim struct {
	UserDBClaimID int        `json:"user_db_claim_id" db:"user_db_claim_id"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	DeletedAt     *time.Time `json:"deleted_at" db:"deleted_at"`

	// start billable fields
	StorageGB int `json:"storage_gb" db:"storage_gb"` // default 10GB storage
	// end billable fields

	Status      string                  `json:"status" db:"status"` // inactive | active | deactivating | activating | error
	Zones       pq.StringArray          `json:"zones" db:"zones"`
	Credentials *UserDBClaimCredentials `json:"credentials" db:"credentials"`
	ProjectID   int                     `json:"project_id" db:"project_id"`
}

type UserDBClaimCredentials struct {
	Credentials []Credentials `json:"credentials" db:"credentials"`
}

type Credentials struct {
	Username          string `json:"username" db:"username"`
	Password          string `json:"password" db:"password"`
	AccessControlType string `json:"access_control_type" db:"access_control_type"`
}

func parseJSONToModel(src interface{}, dest interface{}) error {
	var data []byte

	if b, ok := src.([]byte); ok {
		data = b
	} else if s, ok := src.(string); ok {
		data = []byte(s)
	} else if src == nil {
		return nil
	}

	return json.Unmarshal(data, dest)
}

func (r *UserDBClaimCredentials) Scan(src interface{}) error {
	return parseJSONToModel(src, r)
}

type ObjectStorageClaim struct {
	ObjectStorageClaimID int        `json:"object_storage_claim_id" db:"object_storage_claim_id"`
	CreatedAt            time.Time  `json:"created_at" db:"created_at"`
	DeletedAt            *time.Time `json:"deleted_at" db:"deleted_at"`
	Name                 string     `json:"name" db:"name"`

	// start billable fields
	StorageGB int `json:"storage_gb" db:"storage_gb"` // default 10GB storage
	// end billable fields

	Status    string         `json:"status" db:"status"` // inactive | active | deactivating | activating | error
	Zones     pq.StringArray `json:"zones" db:"zones"`
	ProjectID int            `json:"project_id" db:"project_id"`
}

type ContainerResourceUsagePerAccountPerZone struct {
	ContainerResourceUsagePerAccountPerZoneID int `json:"container_resource_usage_per_account_per_zone_id" db:"container_resource_usage_per_account_per_zone_id"`

	UsedCPUMilliCores int `json:"used_cpu_millicores" db:"used_cpu_millicores"`
	UsedMemoryMB      int `json:"used_memory_mb" db:"used_memory_mb"`

	ZoneName  string `json:"zone_name" db:"zone_name"`
	AccountID int    `json:"account_id" db:"account_id"`
}
