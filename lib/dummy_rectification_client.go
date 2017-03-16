package sous

import (
	//	"github.com/opentable/sous/ext/singularity"
	"log"
)

type (
	// DummyRectificationClient implements RectificationClient but doesn't act on the Mesos scheduler;
	// instead it collects the changes that would be performed and options
	DummyRectificationClient struct {
		logger   *log.Logger
		Created  []dummyRequest
		Deployed []dummyDeploy
		Deleted  []dummyDelete
	}

	dummyDeploy struct {
		Cluster   string
		DepID     string
		ReqID     string
		ImageName string
		Res       Resources
		E         Env
		Vols      Volumes
	}

	dummyRequest struct {
		Cluster string
		ID      string
		Count   int
		Kind    ManifestKind
		Owners  OwnerSet
	}

	dummyDelete struct {
		Cluster, Reqid, Message string
	}
)

// NewDummyRectificationClient builds a new DummyRectificationClient
func NewDummyRectificationClient() *DummyRectificationClient {
	return &DummyRectificationClient{}
}

// SetLogger sets the logger for the client
func (t *DummyRectificationClient) SetLogger(l *log.Logger) {
	l.Println("dummy begin")
	t.logger = l
}

func (t *DummyRectificationClient) log(v ...interface{}) {
	if t.logger != nil {
		t.logger.Print(v...)
	}
}

func (t *DummyRectificationClient) logf(f string, v ...interface{}) {
	if t.logger != nil {
		t.logger.Printf(f, v...)
	}
}

// Deploy implements part of the RectificationClient interface
func (t *DummyRectificationClient) Deploy(
	//	clusterURI, clusterName, depID, reqID, imageName string, res Resources, e Env, vols Volumes, flavor string) error {
	//	deployable Deployable, depID, reqID, imageName string, res Resources, e Env, vols Volumes) error {
	deployable Deployable, reqID string) error {
	clusterURI := deployable.Deployment.Cluster.BaseURL
	clusterName := deployable.Deployment.ClusterName
	flavor := deployable.Deployment.Flavor
	depID := deployable.ComputeDeployID()
	imageName := deployable.BuildArtifact.Name
	res := deployable.Deployment.DeployConfig.Resources
	e := deployable.Deployment.DeployConfig.Env
	vols := deployable.Deployment.DeployConfig.Volumes

	t.logf("Deploying instance %s %s %s %s %s %s %v %v %v", clusterURI, clusterName, flavor, depID, reqID, imageName, res, e, vols)
	t.Deployed = append(t.Deployed, dummyDeploy{clusterURI, depID, reqID, imageName, res, e, vols})
	return nil
}

// PostRequest (cluster, request id, instance count)
func (t *DummyRectificationClient) PostRequest(
	cluster, id string, count int,
	kind ManifestKind,
	owners OwnerSet,
) error {
	t.logf("Creating application %s %s %d %v %v", cluster, id, count, kind, owners)
	t.Created = append(t.Created, dummyRequest{cluster, id, count, kind, owners})
	return nil
}

// DeleteRequest (cluster url, request id, instance count, message)
func (t *DummyRectificationClient) DeleteRequest(
	cluster, reqid, message string) error {
	t.logf("Deleting application %s %s %s", cluster, reqid, message)
	t.Deleted = append(t.Deleted, dummyDelete{cluster, reqid, message})
	return nil
}
