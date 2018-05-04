package graph

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	sous "github.com/opentable/sous/lib"
	"github.com/opentable/sous/util/docker_registry"
	"github.com/opentable/sous/util/logging"
	"github.com/opentable/sous/util/restful"
	"github.com/opentable/sous/util/yaml"
	"github.com/samsalisbury/psyringe"
	"github.com/samsalisbury/semv"
)

type (
	configYAML string

	testConfigLoader struct {
		configYAML
	}
)

const defaultConfig = ""

// DefaultTestGraph results a SousGraph suitable for testing without worrying about details.
func DefaultTestGraph(t *testing.T) *SousGraph {
	stdin := ioutil.NopCloser(bytes.NewReader(nil))
	return BuildTestGraph(t, semv.MustParse("1.1.1"), stdin, ioutil.Discard, ioutil.Discard)
}

// BuildTestGraph builds a standard graph suitable for testing
func BuildTestGraph(t *testing.T, v semv.Version, in io.Reader, out, err io.Writer) *SousGraph {
	return TestGraphWithConfig(t, v, in, out, err, defaultConfig)
}

// TestGraphWithConfig accepts a custom Sous config string
func TestGraphWithConfig(t *testing.T, v semv.Version, in io.Reader, out, err io.Writer, cfg string) *SousGraph {

	graph := BuildGraph(v, in, out, err)

	db := sous.SetupDB(t)
	defer sous.ReleaseDB(t)

	// Add missing stuff to the graph (these are things that come from early
	// initialization in main.go.
	graph.Add(
		func() logging.LogSink { return logging.SilentLogSet() },
	)

	// testGraph methods affect graph as well.
	testGraph := psyringe.TestPsyringe{Psyringe: graph.Psyringe}

	// Replace things from the real graph.
	testGraph.Replace(logging.SilentLogSet())
	testGraph.Replace(sous.User{Name: "Test User", Email: "testuser@example.com"})
	testGraph.Replace(NewTestConfigLoader)
	testGraph.Replace(newDummyDockerClient)
	testGraph.Replace(newServerHandlerFactory)
	testGraph.Replace(newServerStateManager)
	testGraph.Replace(MaybeDatabase{Db: db, Err: nil})

	// Add config.
	testGraph.Add(configYAML(cfg))

	return graph
}

func newDummyHTTPClient() HTTPClient {
	return HTTPClient{HTTPClient: &restful.DummyHTTPClient{}}
}

func newDummyDockerClient() LocalDockerClient {
	return LocalDockerClient{Client: docker_registry.NewDummyClient()}
}

// NewTestConfigLoader produces a faked ConfigLoader
func NewTestConfigLoader(configYAML configYAML) *ConfigLoader {
	cl := &testConfigLoader{configYAML: configYAML}
	return &ConfigLoader{ConfigLoader: cl}
}

func (cl *testConfigLoader) Load(data interface{}, path string) error {
	err := yaml.Unmarshal([]byte(cl.configYAML), data)
	return err
}

func (cl *testConfigLoader) SetValue(interface{}, string, string) error {
	panic("testConfigLoader does not implement SetValue")
}

func (cl *testConfigLoader) SetValidValue(interface{}, string, string) error {
	panic("testConfigLoader does not implement SetValidValue")
}

func (cl *testConfigLoader) GetValue(interface{}, string) (interface{}, error) {
	panic("testConfigLoader does not implement SetValue")
}
