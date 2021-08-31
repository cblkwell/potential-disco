package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/contrib/detectors/aws/ecs"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"google.golang.org/grpc"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	// "go.opentelemetry.io/otel/baggage"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	// "go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
)

const (
	serviceName    = "mtb-app"
	serviceVersion = "0.0.1"
	metricPrefix   = "custom.metric."
	execCountName  = metricPrefix + "exec.count"
	execCountDesc  = "Count of executions."
)

var (
	tracer    trace.Tracer
	meter     metric.Meter
	execCount metric.BoundInt64Counter
)

func main() {

	ctx := context.Background()

	// Get collector endpoint for exporter from env
	otelEndpoint := os.Getenv("EXPORTER_ENDPOINT")

	// Create new OTLP Exporter struct
	driver := otlpgrpc.NewDriver(
		otlpgrpc.WithInsecure(),
		otlpgrpc.WithEndpoint(otelEndpoint),
		otlpgrpc.WithDialOption(grpc.WithBlock()), // useful for testing
	)

	exporter, err := otlp.NewExporter(ctx, driver)
	if err != nil {
		log.Fatalf("%s: %v", "failed to create exporter", err)
	}

	// This generates an X-Ray compatible traceID.
	idg := xray.NewIDGenerator()

	// Instantiate a new ECS resource detector
	ecsResourceDetector := ecs.NewResourceDetector()
	resource, err := ecsResourceDetector.Detect(ctx)
	if err != nil {
		log.Fatalf("%s: %v", "failed to create resource", err)
	}

	// Create a batch span processor -- this batches up spans and exports
	// them to the collector in bulk.
	bsp := sdktrace.NewBatchSpanProcessor(exporter)

	// Create a new trace provider object passing in the config, exporter,
	// ID generator, ECS resource, and a sampler that doesn't actually sample,
	// just exports every trace. These aren't great for production, but good
	// for our tests.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithIDGenerator(idg),
		sdktrace.WithResource(resource),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Create pusher for metrics that runs in the background and pushes
	// metrics every minute.
	pusher := controller.New(
		processor.New(
			simple.NewWithExactDistribution(),
			exporter,
		),
		controller.WithResource(resource),
		controller.WithExporter(exporter),
		controller.WithCollectPeriod(time.Minute),
	)
	err = pusher.Start(ctx)
	if err != nil {
		log.Fatalf("%s: %v", "failed to start the metric controller", err)
	}
	defer func() { _ = pusher.Stop(ctx) }()

	// Set the trace and metric providers and the propagator we want to use.
	// This will allow libraries and other dependencies that use the otel API
	// to emit telemetry data.
	otel.SetTracerProvider(tp)
	global.SetMeterProvider(pusher.MeterProvider())
	otel.SetTextMapPropagator(xray.Propagator{})

	// Create metric instance to support custom metric
	meter = global.Meter("io.opentelemetry.metrics.mtbapp")

	// Creating custom metric to track number of requests to hello endpoint
	execCount = metric.Must(meter).
		NewInt64Counter(
			execCountName,
			metric.WithDescription(execCountDesc),
		).Bind(
		[]attribute.KeyValue{
			attribute.String(
				execCountName,
				execCountDesc)}...)

	// Create a new HTTP router to handle incoming client requests
	r := mux.NewRouter()
	r.Use(otelmux.Middleware(serviceName))

	// When client makes a GET request to /hello, handler will be called
	r.HandleFunc("/hello", hello).Methods(http.MethodGet)
	r.HandleFunc("/dadjoke", dadjoke).Methods(http.MethodGet)

	// Start the server and listen on localhost:8080
	log.Fatal(http.ListenAndServe(":8080", r))
}

// Handler for /hello endpoint
func hello(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()

	// Set the header content-type and return the hello world response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode("hello world")

	// Update execCount metric
	execCount.Add(ctx, 1)
}

type JokeResponse struct {
	ID     string `json:"id"`
	Joke   string `json:"joke"`
	Status int    `json:"status"`
}

// Handler for /dadjoke endpoint
func dadjoke(w http.ResponseWriter, r *http.Request) {

	// ctx := r.Context()

	log.Print("Calling external API for dad jokes")

	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://icanhazdadjoke.com", nil)
	if err != nil {
		log.Print(err.Error())
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Print(err.Error())
	}

	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err.Error())
	}

	var responseObject JokeResponse
	json.Unmarshal(bodyBytes, &responseObject)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responseObject.Joke)
}
