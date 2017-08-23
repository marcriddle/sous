// The goal of this package is to integrate structured logging an metrics
// reporting with error handling in an interface as close as possible to the
// fluency of fmt.Errorf(...)

// or of errors.Wrapf(err, "fmt", args...)

// Concerns:
//   a. structured logging using a defined scheme
//   b. build-time checking of errors
//   c. 3 purposes, which each message type can make use of 1-3 of:
//     logging to ELK,
//     metrics collection
//     error reporting
//   d. Contextualization - i.e. pull message fields from a context.Context
//        or from a logging context likewise contextualized.
//   e. ELK specific fields (i.e. "this is schema xyz")

// Nice to have:
//   z. Output filtering disjoint from creation (i.e. *not* log.debug but rather debug stuff from the singularity API)
//   y. Runtime output filtering, via e.g. HTTP requests.
//   x. A live ringbuffer of all messages

// b & d are in tension.
// also, a with OTLs, because optional fields

package messages

import (
	"runtime"
	"time"

	"github.com/opentable/sous/util/logging"
)

type (
	messageSink interface {
		LogMessage(level, logMessage)
	}

	metricsSink interface {
		GetTimer(name string) logging.Timer
		GetCounter(name string) logging.Counter
		GetUpdater(name string) logging.Updater
	}

	logSink interface {
		messageSink
		metricsSink
	}

	logMessage interface {
		defaultLevel() level
		message() string
		eachField(func(name string, value interface{}))
	}

	metricsMessage interface {
		metricsTo(metricsSink)
	}

	message interface {
	}

	callerInfo struct {
		frame   runtime.Frame
		unknown bool
	}

	level int
	// error interface{}

)

const (
	criticalLevel    = level(iota)
	warningLevel     = level(iota)
	informationLevel = level(iota)
	debugLevel       = level(iota)
	// "extra" debug available
)

// New(name string, ...args) error

// messages.NewClientSendHTTPRequest(serverURL, "./manifest", parms)
// messages.NewClientGotHTTPResponse(serverURL"./manifest", parms, statuscode, body(?), duration)

/*

	messages.WithClientContext(ctx, logger).ReportClientSendHTTPRequest(...)

	// How do we runtime check this without the Context having a specific type?

	clientContext(ctx).LogClientSendHTTPRequest(logger, ...)

	// ^^^ just moves the problem around - clientContext is going to ctx.Value(...).(ClientContext),
	// which can fail at runtime.

	messages.SessionDataFromContext(ctx)
	  -> gets several data items from the ctx...
		-> if any are missing, return a "partialSessionData" which cobbles together a dead letter.


  A static analysis approach here would:

	Check that the JSON tags on structs matched the schemas they claim.
	Check that schema-required fields tie with params to the contructor.
	Maybe check that contexted messages were always receiving contexts with the right WithValues

	A code generation approach would:

	Take the schemas and produce structs with JSON tags
	Produce constructors for the structs with the required fields.
	Produce LogXXX methods and functions around those constructors.

	We can live without those, probably, if we build the interfaces *as if*...

*/

// The plan here is to be able to extend this behavior such that e.g. the rules
// for levels of messages can be configured or updated at runtime.
func getLevel(lm logMessage) level {
	lm.defaultLevel()
}

func getCallerInfo() callerInfo {
	callers := make([]uintptr, 10)
	runtime.Callers(2, callers)
	frames := runtime.CallersFrames(callers)
	for frame, more := frames.Next(); more; frame, more = frames.Next() {
		if strings.Index(frame.File, "util/messages") == -1 {
			return callerInfo{frame: frame}
		}
	}
	return callerInfo{unknown: true}
}

func (info callerInfo) eachField(f func(string, interface{})) {
	if unknown {
		f("file", "<unknown>")
		f("line", "<unknown>")
		f("function", "<unknown>")
		return
	}
	f("file", info.frame.File)
	f("line", info.frame.Line)
	f("function", info.frame.Function)
}

func deliver(message something, logger logSink) {
	if lm, is := message.(logger); is {
		// filtering messages?
		level := getLevel(lm)
		logger.LogMessage(level, lm)
	}

	if mm, is := message.(metricser); is {
		mm.metricsTo(logger)
	}
}

func ReportClientHTTPResponse(logger logSink, server, path string, parms map[string]string, status int, dur time.Duration) {
	m := newClientHTTPResponse(server, path, parms, status, dur)
	deliver(m, logger)
}

type clientHTTPResponse struct {
	partial bool
	Server  string
	Method  string
	Path    string
	Parms   map[string]string
	Status  int
	Dur     time.Duration
}

type clientHTTPResponseSchemaWrapper struct {
	clientHTTPResponse
	SchemaName string `json:"@loglov3-otl"`
}

func newClientHTTPResponse(server, path string, parms map[string]string, status int, dur time.Duration) *clientHTTPResponse {
	return &ClientHTTPResponse{
		Server: server,
		Path:   path,
		Parms:  parms,
		Status: status,
		Dur:    dur,
	}
}
