package jeager

// import (
// 	"io"

// 	"github.com/uber/jaeger-lib/metrics"

// 	"github.com/uber/jaeger-client-go"
// 	jconfig "github.com/uber/jaeger-client-go/config"
// 	jlog "github.com/uber/jaeger-client-go/log"
// )

// var closer io.Closer

// func Init(serviceName string) error {
// 	config := jconfig.Configuration{
// 		Sampler: &jconfig.SamplerConfig{
// 			Type:  jaeger.SamplerTypeConst,
// 			Param: 1,
// 		},
// 		Reporter: &jconfig.ReporterConfig{
// 			LogSpans: true,
// 		},
// 	}

// 	jLogger := jlog.StdLogger
// 	jMetricsFactory := metrics.NullFactory

// 	closer, err := config.InitGlobalTracer(
// 		serviceName,
// 		jconfig.Logger(jLogger),
// 		jconfig.Metrics(jMetricsFactory),
// 	)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }
