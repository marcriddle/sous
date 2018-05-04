package graph

import (
	"database/sql"
	"fmt"
	"io"
	"io/ioutil" //ok
	"net/http"
	"os"
	"os/user"

	"github.com/opentable/sous/config"
	"github.com/opentable/sous/ext/docker"
	"github.com/opentable/sous/ext/git"
	"github.com/opentable/sous/ext/github"
	"github.com/opentable/sous/ext/singularity"
	"github.com/opentable/sous/ext/storage"
	"github.com/opentable/sous/lib"
	"github.com/opentable/sous/server"
	"github.com/opentable/sous/util/docker_registry"
	"github.com/opentable/sous/util/logging"
	"github.com/opentable/sous/util/logging/messages"
	"github.com/opentable/sous/util/restful"
	"github.com/opentable/sous/util/shell"
	"github.com/pkg/errors"
	"github.com/samsalisbury/psyringe"
	"github.com/samsalisbury/semv"
	uuid "github.com/satori/go.uuid"
)

type (
	// SousGraph is a dependency injector used to flesh out Sous commands
	// with their dependencies.
	SousGraph struct {
		addGuards map[string]bool
		*psyringe.Psyringe
	}
	// OutWriter is typically set to os.Stdout.
	OutWriter io.Writer
	// ErrWriter is typically set to os.Stderr.
	ErrWriter io.Writer
	// InReader is typicially set to os.Stdin
	InReader io.Reader
	// StatusWaitStable represents if `sous plumbing status` should continue to
	// poll until the selected t
	StatusWaitStable bool
	// ProfilingServer records whether a profiling server was requested
	ProfilingServer bool

	// LocalSousConfig is the configuration for Sous.
	LocalSousConfig struct {
		*config.Config
		LogSink LogSink
	}
	// LocalWorkDir is the user's current working directory when they invoke Sous.
	LocalWorkDir string
	// LocalWorkDirShell is a shell for working in the user's current working
	// directory.
	LocalWorkDirShell struct{ *shell.Sh }
	// LocalGitClient is a git client rooted in WorkdirShell.Dir.
	LocalGitClient struct{ *git.Client }
	// LocalGitRepo is the git repository containing WorkDir.
	LocalGitRepo struct{ *git.Repo }
	// ScratchDirShell is a shell for working in the scratch area where things
	// like artefacts, and build metadata are stored. It is a new, empty
	// directory, and should be cleaned up eventually.
	ScratchDirShell struct{ *shell.Sh }
	// LocalDockerClient is a docker client object
	LocalDockerClient struct{ docker_registry.Client }
	// HTTPClient wraps the sous.HTTPClient interface
	HTTPClient struct{ restful.HTTPClient }
	// ClientBundle collects HTTPClients per server.
	ClientBundle map[string]restful.HTTPClient
	// ClusterSpecificHTTPClient wraps the sous.HTTPClient interface
	ClusterSpecificHTTPClient struct{ restful.HTTPClient }
	// ServerHandler wraps the http.Handler for the sous server
	ServerHandler struct{ http.Handler }
	// MetricsHandler wraps an http.Handler for metrics
	MetricsHandler struct{ http.Handler }
	// LogSink wraps logging.LogSink
	LogSink struct{ logging.LogSink }
	// DefaultLogSink depends only on a semv.Version so can be used prior to reading
	// any configuration.
	DefaultLogSink struct{ logging.LogSink }
	// ClusterManager simply wraps the sous.ClusterManager interface
	ClusterManager struct{ sous.ClusterManager }
	// StateManager simply wraps the sous.StateManager interface
	StateManager struct{ sous.StateManager }
	// ServerStateManager simply wraps the sous.StateManager interface
	ServerStateManager struct{ sous.StateManager }
	// StateReader wraps a storage.StateReader.
	StateReader struct{ sous.StateReader }
	// StateWriter wraps a storage.StateWriter, and should be configured to
	// use the current user's local storage.
	StateWriter struct{ sous.StateWriter }
	// CurrentGDM is a snapshot of the GDM at application start. In a CLI
	// context, which this is, that is all we need to simply read the GDM.
	CurrentGDM struct{ sous.Deployments }
	// TargetManifest is a specific manifest for the current ManifestID.
	// If the named manifest does not exist, it is created.
	TargetManifest struct{ *sous.Manifest }
	// detectedOTPLDeployManifest is a set of otpl-deploy configured deployments
	// that have been detected.
	detectedOTPLDeployManifest struct{ sous.Manifests }
	// userSelectedOTPLDeployManifest is a set of otpl-deploy configured deploy
	// specs that the user has explicitly selected. (May be empty.)
	userSelectedOTPLDeployManifest struct{ *sous.Manifest }
	// TargetDeploymentID is the manifest ID being targeted, after resolving all
	// context and flags.
	TargetDeploymentID sous.DeploymentID
	// TargetManifestID is the manifest ID being targeted, after resolving all
	// context and flags.
	TargetManifestID sous.ManifestID
	// DryrunOption specifies components that should be faked in an execution.
	DryrunOption string
	// SourceContextDiscovery captures the possiblity of not finding a SourceContext
	SourceContextDiscovery struct {
		Error error
		*sous.ManifestID
		*sous.SourceContext
	}
	// ServerListData collects responses from /servers
	ServerListData struct {
		Servers []struct {
			ClusterName string
			URL         string
		}
	}
)

const (
	// DryrunBoth specifies that both the registry and scheduler should be fake.
	DryrunBoth = DryrunOption("both")
	// DryrunRegistry means the registry should be faked for this execution.
	DryrunRegistry = DryrunOption("registry")
	// DryrunScheduler means the scheduler should be faked for this execution.
	DryrunScheduler = DryrunOption("scheduler")
	// DryrunNeither means neither the registry or scheduler should be faked.
	DryrunNeither = DryrunOption("none")
)

// BuildGraph builds the dependency injection graph, used to populate commands
// invoked by the user.
func BuildGraph(v semv.Version, in io.Reader, out, err io.Writer) *SousGraph {
	graph := BuildBaseGraph(v, in, out, err)
	AddFilesystem(graph)
	AddNetwork(graph)
	AddState(graph)
	graph.Add(newUser)
	return graph
}

func newUser(c LocalSousConfig) sous.User {
	return c.User
}

func newSousGraph() *SousGraph {
	return &SousGraph{
		addGuards: map[string]bool{},
		Psyringe:  psyringe.New(),
	}
}

// BuildBaseGraph constructs a graph with essentials - intended for testing
func BuildBaseGraph(version semv.Version, in io.Reader, out, err io.Writer) *SousGraph {
	graph := newSousGraph()
	graph.Add(
		version,
		sous.TraceID(uuid.NewV4().String()),
		func() InReader { return in },
		func() OutWriter { return out },
		func() ErrWriter { return err },
	)

	AddLogs(graph)
	AddUser(graph)
	AddShells(graph)
	AddConfig(graph)
	AddDocker(graph)
	AddSingularity(graph)
	AddInternals(graph)
	graph.Add(graph)
	return graph
}

type adder interface {
	Add(...interface{})
}

// AddLogs adds a logset to the graph.
func AddLogs(graph adder) {
	graph.Add(
		newLogSet,
		newLogSink,
		newDefaultLogSink,
		newMetricsHandler,
	)
}

// AddUser adds the OS user to the graph.
func AddUser(graph adder) {
	graph.Add(
		newLocalUser,
	)
}

// AddShells adds working shells to the graph.
func AddShells(graph adder) {
	graph.Add(
		newLocalWorkDirShell,
		newScratchDirShell,
	)
}

// AddFilesystem adds filesystem to the graph.
func AddFilesystem(graph adder) {
	graph.Add(
		newConfigLoader,
		newMaybeDatabase, // we need to be able to progress in the absence of a DB.
		newServerStateManager,
	)
}

// AddConfig adds filesystem to the graph.
func AddConfig(graph adder) {
	c := config.DefaultConfig()
	graph.Add(
		DefaultConfig{&c},
		newRawConfig,
		newPossiblyInvalidLocalSousConfig,
		newLocalSousConfig,
		newSousConfig,
		newLocalWorkDir,
	)
}

// AddNetwork adds features that require the network.
func AddNetwork(graph adder) {
	graph.Add(
		newDockerClient,
		newServerHandlerFactory,
		newServerHandler,
		newHTTPStateManager,
	)
}

// AddDocker adds Docker to the graph.
func AddDocker(graph adder) {
	graph.Add(
		newLazyNameCache,
		newNameCache,
		newDockerBuilder,
		newSelector,
	)
}

// AddSingularity adds Singularity clients to the graph.
func AddSingularity(graph adder) {
	graph.Add(
		newDeployer,
	)
}

// AddState adds state reader and writers to the graph.
func AddState(graph adder) {
	graph.Add(
		newStateManager,
		newLocalStateReader,
		newLocalStateWriter,
	)
}

// AddInternals adds the dependency contructors that are internal to Sous.
func AddInternals(graph adder) {
	// internal to Sous
	graph.Add(
		newRegistryDumper,
		newRegistry,
		newLabeller,
		newRegistrar,
		newBuildManager,
		newBuildConfig,
		newBuildContext,
		newSourceContext,
		newSourceContextDiscovery,
		newSourceHostChooser,
		NewCurrentState,
		NewCurrentGDM,
		newTargetManifest,
		newDetectedOTPLConfig,
		newUserSelectedOTPLDeploySpecs,
		newRefinedResolveFilter,
		newTargetManifestID,
		newTargetDeploymentID,
		newResolveFilter,
		newResolver,
		newAutoResolver,
		newInserter,
		newStatusPoller,
		newServerComponentLocator,
		newHTTPClient,
		newServerListData,
		newHTTPClientBundle,
		newClusterSpecificHTTPClient,
		NewR11nQueueSet,
		newQueueSet,
	)
}

func newResolveFilter(sf *config.DeployFilterFlags, shc sous.SourceHostChooser) (*sous.ResolveFilter, error) {
	return sf.BuildFilter(shc.ParseSourceLocation)
}

func newResolver(filter *sous.ResolveFilter, d sous.Deployer, r sous.Registry, ls LogSink, qs *sous.R11nQueueSet) *sous.Resolver {
	return sous.NewResolver(d, r, filter, ls.Child("resolver"), qs)
}

func newAutoResolver(rez *sous.Resolver, sr *ServerStateManager, ls LogSink) *sous.AutoResolver {
	return sous.NewAutoResolver(rez, sr, ls.Child("autoresolver"))
}

func newSourceHostChooser() sous.SourceHostChooser {
	return sous.SourceHostChooser{
		SourceHosts: []sous.SourceHost{
			github.SourceHost{},
		},
	}
}

func newRegistryDumper(r sous.Registry) *sous.RegistryDumper {
	return sous.NewRegistryDumper(r)
}

// newLogSet relies only on PossiblyInvalidConfig because we need to initialise
// logging very early on, but don't want to break other commands that do not
// rely on valid configuration (especially 'sous config' which explicitly needs
// to handle broken config in order to allow fixing it).
//
// If handed invalid config, we emit a warning on stderr and proceed with a
// default LogSet.
func newLogSet(v semv.Version, config PossiblyInvalidConfig, tid sous.TraceID) (*logging.LogSet, error) {
	ls := logging.NewLogSet(v, "", "", os.Stderr, tid)
	if configErr := config.Logging.Validate(); configErr != nil {
		// No need to warn here, this is handled by PIC constructor.
		config.Logging = logging.Config{}
	}

	if err := ls.Configure(config.Logging); err != nil {
		return ls, initErr(err, "validating logging configuration")
	}
	if err := logging.Log.Configure(config.Logging); err != nil {
		return ls, initErr(err, "validating logging configuration")
	}
	return ls, nil
}

func newDefaultLogSink(v semv.Version) DefaultLogSink {
	return DefaultLogSink{LogSink: logging.NewLogSet(v, "", "", os.Stderr)}
}

func newLogSink(v *config.Verbosity, set *logging.LogSet) LogSink {
	//set.Configure(v.LoggingConfiguration())
	v.UpdateLevel(set)

	logging.ReportMsg(set, logging.InformationLevel, "Info debugging enabled")
	logging.ReportMsg(set, logging.ExtraDebug1Level, "Verbose debugging enabled")
	logging.ReportMsg(set, logging.DebugLevel, "Regular debugging enabled")
	return LogSink{set}
}

func newMetricsHandler(set *logging.LogSet) MetricsHandler {
	return MetricsHandler{set.ExpHandler()}
}

func newSourceContextDiscovery(sh LocalWorkDirShell, ls LogSink) *SourceContextDiscovery {
	var err error

	gitc := LocalGitClient{}
	gitc.Client, err = git.NewClient(sh.Sh)
	if err != nil {
		return &SourceContextDiscovery{
			Error:         err,
			SourceContext: nil,
		}
	}

	g := LocalGitRepo{}
	g.Repo, err = gitc.OpenRepo(".")
	if err != nil {
		return &SourceContextDiscovery{
			Error:         err,
			SourceContext: nil,
		}
	}

	c, err := g.SourceContext()
	if err != nil {
		return &SourceContextDiscovery{
			Error:         err,
			SourceContext: nil,
		}
	}
	detected := c.NearestTagName
	annotated, err := g.Client.NearestAnnotatedTag()

	logging.Deliver(ls, logging.NewGenericMsg(logging.InformationLevel, "source context tag", map[string]interface{}{
		"detected-tag":              detected,
		"nearest-annotated-tag":     annotated,
		"detected-equals-annotated": (detected == annotated),
	}, false))

	return &SourceContextDiscovery{
		Error:         err,
		SourceContext: c,
	}
}

func newSourceContext(mid TargetManifestID, discovered *SourceContextDiscovery) (*sous.SourceContext, error) {
	return discovered.Unwrap(mid)
}

// Unwrap returns the SourceContext and the returned error in trying to create it.
func (scd *SourceContextDiscovery) Unwrap(mid TargetManifestID) (*sous.SourceContext, error) {
	if scd.Error != nil {
		return nil, scd.Error
	}
	sl := sous.ManifestID(mid)
	if sl.Source.Repo != scd.SourceLocation().Repo {
		return nil, errors.Errorf("source location %q is not the same as the remote %q", sl.Source.Repo, scd.SourceLocation().Repo)
	}
	return scd.SourceContext, nil
}

// GetContext returns the SourceContext discovered if there were no errors in
// getting it. Otherwise returns a pointer to a zero SourceContext.
func (scd *SourceContextDiscovery) GetContext() *sous.SourceContext {
	if scd.Error != nil || scd.SourceContext == nil {
		return &sous.SourceContext{}
	}
	return scd.SourceContext
}

func newBuildContext(wd LocalWorkDirShell, c *sous.SourceContext) *sous.BuildContext {
	sh := wd.Sh.Clone()
	sh.LongRunning(true)
	return &sous.BuildContext{Sh: sh, Source: *c}
}

func newBuildConfig(ls LogSink, f *config.DeployFilterFlags, p *config.PolicyFlags, bc *sous.BuildContext) *sous.BuildConfig {
	offset := f.Offset
	if offset == "" {
		offset = bc.Source.OffsetDir
	}
	cfg := sous.BuildConfig{
		Repo:       f.Repo,
		Offset:     offset,
		Tag:        f.Tag,
		Revision:   f.Revision,
		Strict:     p.Strict,
		ForceClone: p.ForceClone,
		Dev:        p.Dev,
		Context:    bc,
		LogSink:    ls,
	}
	cfg.Resolve()

	return &cfg
}

func newBuildManager(ls LogSink, bc *sous.BuildConfig, sl sous.Selector, lb sous.Labeller, rg sous.Registrar) *sous.BuildManager {
	return &sous.BuildManager{
		BuildConfig: bc,
		Selector:    sl,
		Labeller:    lb,
		Registrar:   rg,
		LogSink:     ls,
	}
}

func newLocalUser() (v config.LocalUser, err error) {
	u, err := user.Current()
	return config.LocalUser{User: u}, initErr(err, "getting current user")
}

// TODO: This should register a cleanup task with the cli, to delete the temp
// dir.
func newScratchDirShell() (v ScratchDirShell, err error) {
	const what = "getting scratch directory"
	dir, err := ioutil.TempDir("", "sous")
	if err != nil {
		return v, initErr(err, what)
	}
	v.Sh, err = shell.DefaultInDir(dir)
	v.TeeOut = os.Stdout
	v.TeeErr = os.Stderr
	return v, initErr(err, what)
}

func newLocalWorkDir() (LocalWorkDir, error) {
	s, err := os.Getwd()
	return LocalWorkDir(s), initErr(err, "determining working directory")
}

func newLocalWorkDirShell(verbosity *config.Verbosity, l LocalWorkDir) (v LocalWorkDirShell, err error) {
	v.Sh, err = shell.DefaultInDir(string(l))
	v.TeeEcho = os.Stdout //XXX should use a writer
	v.Sh.Debug = verbosity.Debug
	return v, initErr(err, "getting current working directory")
}

func newSelector(regClient LocalDockerClient, log LogSink) sous.Selector {
	return docker.NewBuildStrategySelector(log.Child("docker-build-strategy"), regClient)
}

func newDockerBuilder(cfg LocalSousConfig, nc sous.Inserter, ctx *sous.SourceContext, source LocalWorkDirShell, scratch ScratchDirShell) (*docker.Builder, error) {
	drh := cfg.Docker.RegistryHost
	source.Sh = source.Sh.Clone().(*shell.Sh)
	source.Sh.LongRunning(true)
	return docker.NewBuilder(nc, drh, source.Sh, scratch.Sh)
}

func newLabeller(db *docker.Builder) sous.Labeller {
	return db
}

func newRegistrar(db *docker.Builder) sous.Registrar {
	return db
}

func newRegistry(graph *SousGraph, nc lazyNameCache, dryrun DryrunOption, c LocalSousConfig) (sous.Registry, error) {
	// We only need a real registry when running in server or workstation mode.
	if c.Server == "" && dryrun != DryrunBoth && dryrun != DryrunRegistry {
		return nc()
	}
	return sous.NewDummyRegistry(), nil
}

func newDeployer(dryrun DryrunOption, nc lazyNameCache, ls LogSink, c LocalSousConfig) (sous.Deployer, error) {
	// Eventually, based on configuration, we may make different decisions here.
	if dryrun == DryrunBoth || dryrun == DryrunScheduler || c.Server != "" {
		drc := sous.NewDummyRectificationClient()
		drc.SetLogger(ls.Child("rectify"))
		return singularity.NewDeployer(
			drc,
			ls.Child("singularity-deployer"),
			singularity.OptMaxHTTPReqsPerServer(c.MaxHTTPConcurrencySingularity),
		), nil
	}
	// We need the real name cache.
	labeller, err := nc()
	if err != nil {
		return nil, err
	}
	return singularity.NewDeployer(
		singularity.NewRectiAgent(labeller),
		ls,
		singularity.OptMaxHTTPReqsPerServer(c.MaxHTTPConcurrencySingularity),
	), nil
}

// ServerHandlerFactory produces a ServerHandler when invoked.
type ServerHandlerFactory func() (ServerHandler, error)

func newServerHandlerFactory(g *SousGraph, metrics MetricsHandler, log LogSink) ServerHandlerFactory {
	return func() (ServerHandler, error) {

		clScoop := struct {
			LogSink
			LocalSousConfig
			sous.Inserter
			*ServerStateManager
			*sous.ResolveFilter
			*sous.AutoResolver
			semv.Version
			*sous.R11nQueueSet
		}{}

		if err := g.Inject(&clScoop); err != nil {
			return ServerHandler{}, err
		}

		cl := newServerComponentLocator(
			clScoop.LogSink,
			clScoop.LocalSousConfig,
			clScoop.Inserter,
			clScoop.ServerStateManager,
			clScoop.ResolveFilter,
			clScoop.AutoResolver,
			clScoop.Version,
			clScoop.R11nQueueSet,
		)

		var handler http.Handler

		profileQuery := struct{ Yes ProfilingServer }{}
		g.Inject(&profileQuery)
		if profileQuery.Yes {
			handler = server.ProfilingHandler(cl, metrics, log.Child("http-server"))
		} else {
			handler = server.Handler(cl, metrics, log.Child("http-server"))
		}

		return ServerHandler{handler}, nil
	}
}

func newServerHandler(f ServerHandlerFactory) (ServerHandler, error) {
	return f()
}

func newServerListData(c HTTPClient) (ServerListData, error) {
	serverList := ServerListData{}
	_, err := c.Retrieve("./servers", nil, &serverList, nil)
	return serverList, err
}

func newHTTPClientBundle(serverList ServerListData, tid sous.TraceID, log LogSink) (ClientBundle, error) {
	bundle := ClientBundle{}
	for _, s := range serverList.Servers {
		client, err := restful.NewClient(s.URL, log.Child(s.ClusterName+".http-client"), map[string]string{"OT-RequestId": string(tid)})
		if err != nil {
			return nil, err
		}

		bundle[s.ClusterName] = client
	}
	return bundle, nil
}

// newClusterSpecificHTTPClient returns an HTTP client configured to talk to
// the cluster defined by DeployFilterFlags.
// Otherwise it returns nil, and emits some warnings.
func newClusterSpecificHTTPClient(clients ClientBundle, rf *sous.ResolveFilter, log LogSink) (*ClusterSpecificHTTPClient, error) {
	cluster, err := rf.Cluster.Value()
	if err != nil {
		return nil, fmt.Errorf("Setting up HTTP client: cluster: %s", err) // errors.Wrapf && cli don't play nice
	}

	cl, has := clients[cluster]
	if !has {
		return nil, fmt.Errorf("no server for cluster %q", cluster)
	}
	return &ClusterSpecificHTTPClient{HTTPClient: cl}, nil
}

// newHTTPClient returns an HTTP client if c.Server is not empty.
// Otherwise it returns nil, and emits some warnings.
func newHTTPClient(c LocalSousConfig, user sous.User, srvr ServerHandlerFactory, tid sous.TraceID, log LogSink) (HTTPClient, error) {
	if c.Server == "" {
		messages.ReportLogFieldsMessageToConsole("No server set, Sous is running in server or workstation mode.", logging.WarningLevel, log)
		messages.ReportLogFieldsMessageToConsole("Configure a server like this: sous config server http://some.sous.server", logging.WarningLevel, log)
		s, err := srvr()
		if err != nil {
			return HTTPClient{}, err
		}
		cl, err := restful.NewInMemoryClient(s.Handler, log.Child("local-http"))
		return HTTPClient{HTTPClient: cl}, err
	}
	messages.ReportLogFieldsMessageToConsole(fmt.Sprintf("Using server %s", c.Server), logging.ExtraDebug1Level, log)
	cl, err := restful.NewClient(c.Server, log.Child("http-client"), map[string]string{"OT-RequestId": string(tid)})
	return HTTPClient{HTTPClient: cl}, err
}

func newServerStateManager(c LocalSousConfig, mdb MaybeDatabase, rf *sous.ResolveFilter, log LogSink) (*ServerStateManager, error) {
	var secondary sous.StateManager
	var err error
	if mdb.Err == nil {
		secondary, err = newDistributedStorage(mdb.Db, c, rf, log)
		if err != nil {
			return nil, err
		}

	} else {
		err = mdb.Err
		// Either DB not configured, or problems setting up dispatcher...
		logging.ReportError(log, errors.Wrapf(err, "connecting to database with %#v", c.Database))
		secondary = storage.NewLogOnlyStateManager(log.Child("database"))
	}

	dm := storage.NewDiskStateManager(c.StateLocation)
	gm := storage.NewGitStateManager(dm)
	duplex := storage.NewDuplexStateManager(gm, secondary, log.Child("duplex-state"))
	return &ServerStateManager{StateManager: duplex}, nil
}

func newDistributedStorage(db *sql.DB, c LocalSousConfig, rf *sous.ResolveFilter, log LogSink) (sous.StateManager, error) {
	localName, err := rf.Cluster.Value()
	if err != nil {
		return nil, fmt.Errorf("Setting up distributed storage: cluster: %s", err) // errors.Wrapf && cli don't play nice
	}

	local := storage.NewPostgresStateManager(db, log.Child("database"))
	list := ClientBundle{}
	clusterNames := []string{}
	for n, u := range c.SiblingURLs {
		// XXX not immediately clear how to conserve the request id through the distributed storage.
		cl, err := restful.NewClient(u, log.Child(n+".http-client"))
		if err != nil {
			return nil, err
		}
		list[n] = cl
		clusterNames = append(clusterNames, n)
	}
	hsm := sous.NewHTTPStateManager(list[localName], list)
	return sous.NewDispatchStateManager(localName, clusterNames, local, hsm), nil
}

// newStateManager returns a wrapped sous.HTTPStateManager if cl is not nil.
// Otherwise it returns a wrapped sous.GitStateManager, for local git based GDM.
// If it returns a sous.GitStateManager, it emits a warning log.
func newStateManager(cl HTTPClient, c LocalSousConfig, mdb MaybeDatabase, bundle ClientBundle, rf *sous.ResolveFilter, log LogSink) *StateManager {
	if c.Server == "" {
		messages.ReportLogFieldsMessageToConsole(fmt.Sprintf("Using local state stored at %s", c.StateLocation), logging.WarningLevel, log, c.StateLocation)
		ssm, err := newServerStateManager(c, mdb, rf, log)
		if err != nil {
			panic(err)
		}
		return &StateManager{StateManager: ssm.StateManager}
	}
	hsm := sous.NewHTTPStateManager(cl, bundle)
	return &StateManager{StateManager: hsm}
}

func newHTTPStateManager(cl HTTPClient, bundle ClientBundle) *sous.HTTPStateManager {
	return sous.NewHTTPStateManager(cl, bundle)
}

func newStatusPoller(cl HTTPClient, rf *RefinedResolveFilter, user sous.User, logs LogSink) *sous.StatusPoller {
	if cl.HTTPClient == nil {
		messages.ReportLogFieldsMessageToConsole("Unable to poll for status.", logging.WarningLevel, logs, rf)
		return nil
	}
	messages.ReportLogFieldsMessageToConsole("...looks good...", logging.ExtraDebug1Level, logs)
	return sous.NewStatusPoller(cl, (*sous.ResolveFilter)(rf), user, logs.Child("status-poller"))
}

func newLocalStateReader(sm *StateManager) StateReader {
	return StateReader{sm}
}

func newLocalStateWriter(sm *StateManager) StateWriter {
	return StateWriter{sm}
}

// NewCurrentState returns the current *sous.State.
func NewCurrentState(sr StateReader, log LogSink) (*sous.State, error) {
	state, err := sr.ReadState()
	if os.IsNotExist(errors.Cause(err)) || storage.IsGSMError(err) {
		messages.ReportLogFieldsMessageToConsole("error reading state, defaulting to empty state", logging.WarningLevel, log, err)
		return sous.NewState(), nil
	}
	return state, initErr(err, "reading sous state")
}

// NewCurrentGDM returns the current GDM.
func NewCurrentGDM(state *sous.State) (CurrentGDM, error) {
	deployments, err := state.Deployments()
	if err != nil {
		return CurrentGDM{}, initErr(err, "expanding state")
	}
	return CurrentGDM{deployments}, initErr(err, "expanding state")
}

// The funcs named makeXXX below are used to create specific implementations of
// sous native types.

func newInserter(cfg LocalSousConfig, nc lazyNameCache, tid sous.TraceID, log LogSink) (sous.Inserter, error) {
	if cfg.Server == "" {
		return nc()
	}
	cl, err := restful.NewClient(cfg.Server, log.Child("http-client"), map[string]string{"OT-RequestId": string(tid)})
	return sous.NewHTTPNameInserter(cl, tid, log.LogSink), err
}

// initErr returns nil if error is nil, otherwise an initialisation error.
// The second argument "what" should be a very short description of the
// initialisation task, e.g. "getting widget" or "reading state" etc.
func initErr(err error, what string) error {
	if err == nil {
		return nil
	}
	message := fmt.Sprintf("error %s:", what)
	if shellErr, ok := err.(shell.Error); ok {
		message += fmt.Sprintf("\ncommand failed:\nshell> %s\n%s",
			shellErr.Command.String(), shellErr.Result.Combined.String())
	} else {
		message += " " + err.Error()
	}
	return fmt.Errorf(message)
}
