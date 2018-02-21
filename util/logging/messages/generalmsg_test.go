package messages

import (
	"fmt"
	"testing"

	"github.com/opentable/sous/util/logging"
)

func TestReportLogFieldsMessage_Complete(t *testing.T) {
	logging.AssertReportFields(t,
		func(ls logging.LogSink) {
			cfg := logging.Config{}
			cfg.Kafka.Topic = "test-topic"
			cfg.Kafka.BrokerList = "broker1,broker2,broker3"

			ReportLogFieldsMessage("This is test message", logging.DebugLevel, ls, cfg)
		},
		logging.StandardVariableFields,
		map[string]interface{}{
			"fields":       "Basic,Kafka,Graphite,Config,Level,DisableConsole,Enabled,DefaultLevel,Topic,Brokers,BrokerList,Server",
			"types":        "Config,string,bool",
			"json-value":   "{\"message\":{\"array\":[\"(logging.Config) {\\n Basic: (struct { Level string \\\"env:\\\\\\\"SOUS_LOGGING_LEVEL\\\\\\\"\\\"; DisableConsole bool }) {\\n  Level: (string) \\\"\\\",\\n  DisableConsole: (bool) false\\n },\\n Kafka: (struct { Enabled bool; DefaultLevel string \\\"env:\\\\\\\"SOUS_KAFKA_LOG_LEVEL\\\\\\\"\\\"; Topic string \\\"env:\\\\\\\"SOUS_KAFKA_TOPIC\\\\\\\"\\\"; Brokers []string; BrokerList string \\\"env:\\\\\\\"SOUS_KAFKA_BROKERS\\\\\\\"\\\" }) {\\n  Enabled: (bool) false,\\n  DefaultLevel: (string) \\\"\\\",\\n  Topic: (string) (len=10) \\\"test-topic\\\",\\n  Brokers: ([]string) \\u003cnil\\u003e,\\n  BrokerList: (string) (len=23) \\\"broker1,broker2,broker3\\\"\\n },\\n Graphite: (struct { Enabled bool; Server string \\\"env:\\\\\\\"SOUS_GRAPHITE_SERVER\\\\\\\"\\\" }) {\\n  Enabled: (bool) false,\\n  Server: (string) \\\"\\\"\\n }\\n}\\n\"]}}",
			"@loglov3-otl": "sous-generic-v1",
		})
}

func TestReportLogFieldsMessage_NoInterface(t *testing.T) {
	logging.AssertReportFields(t,
		func(ls logging.LogSink) {
			ReportLogFieldsMessage("This is test message no interface", logging.DebugLevel, ls)
		},
		logging.StandardVariableFields,
		map[string]interface{}{
			"fields":       "",
			"types":        "",
			"json-value":   "{\"message\":{\"array\":[]}}",
			"@loglov3-otl": "sous-generic-v1",
		})
}
func TestReportLogFieldsMessage_String(t *testing.T) {
	logging.AssertReportFields(t,
		func(ls logging.LogSink) {
			ReportLogFieldsMessage("This is test message passing just a string", logging.DebugLevel, ls, "simple string")
		},
		logging.StandardVariableFields,
		map[string]interface{}{
			"fields":       "",
			"types":        "string",
			"json-value":   "{\"message\":{\"array\":[\"{\\\"string\\\":{\\\"string\\\":\\\"simple string\\\"}}\"]}}",
			"@loglov3-otl": "sous-generic-v1",
		})
}

func TestReportLogFieldsMessage_StructAndString(t *testing.T) {
	logging.AssertReportFields(t,
		func(ls logging.LogSink) {
			cfg := logging.Config{}
			cfg.Kafka.Topic = "test-topic"
			cfg.Kafka.BrokerList = "broker1,broker2,broker3"

			ReportLogFieldsMessage("This is test message", logging.DebugLevel, ls, cfg, "simple string")
		},
		logging.StandardVariableFields,
		map[string]interface{}{
			"fields":       "Basic,Kafka,Graphite,Config,Level,DisableConsole,Enabled,DefaultLevel,Topic,Brokers,BrokerList,Server",
			"types":        "Config,string,bool",
			"json-value":   "{\"message\":{\"array\":[\"(logging.Config) {\\n Basic: (struct { Level string \\\"env:\\\\\\\"SOUS_LOGGING_LEVEL\\\\\\\"\\\"; DisableConsole bool }) {\\n  Level: (string) \\\"\\\",\\n  DisableConsole: (bool) false\\n },\\n Kafka: (struct { Enabled bool; DefaultLevel string \\\"env:\\\\\\\"SOUS_KAFKA_LOG_LEVEL\\\\\\\"\\\"; Topic string \\\"env:\\\\\\\"SOUS_KAFKA_TOPIC\\\\\\\"\\\"; Brokers []string; BrokerList string \\\"env:\\\\\\\"SOUS_KAFKA_BROKERS\\\\\\\"\\\" }) {\\n  Enabled: (bool) false,\\n  DefaultLevel: (string) \\\"\\\",\\n  Topic: (string) (len=10) \\\"test-topic\\\",\\n  Brokers: ([]string) \\u003cnil\\u003e,\\n  BrokerList: (string) (len=23) \\\"broker1,broker2,broker3\\\"\\n },\\n Graphite: (struct { Enabled bool; Server string \\\"env:\\\\\\\"SOUS_GRAPHITE_SERVER\\\\\\\"\\\" }) {\\n  Enabled: (bool) false,\\n  Server: (string) \\\"\\\"\\n }\\n}\\n\",\"{\\\"string\\\":{\\\"string\\\":\\\"simple string\\\"}}\"]}}",
			"@loglov3-otl": "sous-generic-v1",
		})
}

//normally wouldn't use this logger with http response, but this was just done to test logging of a very complex structure
//and ensure it didn't fail ot going to put json-value as expected, since it contains pointers that can change on run
//execution
func TestReportLogFieldsMessage_TwoStructs(t *testing.T) {
	logging.AssertReportFields(t,
		func(ls logging.LogSink) {
			cfg := logging.Config{}
			cfg.Kafka.Topic = "test-topic"
			cfg.Kafka.BrokerList = "broker1,broker2,broker3"
			res := buildHTTPResponse(t, "GET", "http://example.com/api?a=a", 200, 0, 123)
			ReportLogFieldsMessage("This is test message", logging.DebugLevel, ls, cfg, res)
		},
		append(logging.StandardVariableFields, "json-value"),
		map[string]interface{}{
			"fields":       "Basic,Kafka,Graphite,Config,Level,DisableConsole,Enabled,DefaultLevel,Topic,Brokers,BrokerList,Server,Status,StatusCode,Proto,ProtoMajor,ProtoMinor,Header,Body,ContentLength,TransferEncoding,Close,Uncompressed,Trailer,Request,TLS,Response", //nolint
			"types":        "Config,string,bool,*Response,int,Header,int64,*Request,*ConnectionState",
			"@loglov3-otl": "sous-generic-v1",
		})
}

func TestReportLogFieldsMessage_CyclicalReference(t *testing.T) {
	logging.AssertReportFields(t,
		func(ls logging.LogSink) {
			type Parent struct {
				Child   *Parent
				LogData string
			}

			myVar := Parent{}
			myVar.LogData = "Hello"
			myVar.Child = &myVar
			ReportLogFieldsMessageToConsole("This is test message", logging.DebugLevel, ls, myVar)
		},
		append(logging.StandardVariableFields, "json-value"),
		map[string]interface{}{
			"fields":       "Child,LogData,Parent",
			"types":        "Parent,*Parent,string",
			"@loglov3-otl": "sous-generic-v1",
		})
}

func TestReportLogFieldsMessage_error(t *testing.T) {
	logging.AssertReportFields(t,
		func(ls logging.LogSink) {

			err := fmt.Errorf("error msg")
			ReportLogFieldsMessage("This is test message", logging.DebugLevel, ls, err)
		},
		logging.StandardVariableFields,
		map[string]interface{}{
			"fields":       "",
			"types":        "error",
			"json-value":   "{\"message\":{\"array\":[\"{\\\"error\\\":{\\\"error\\\":\\\"error msg\\\"}}\"]}}",
			"@loglov3-otl": "sous-generic-v1",
		})
}