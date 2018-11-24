package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
)

var version string
var cfg config

type config struct {
	verbosity bool
}

func handleErr(err error) {
	if cfg.verbosity {
		fmt.Println(err)
	}
}

func parseCWDimensions(cwDimensions string) (dims []*cloudwatch.Dimension) {
	// example input string is `Host=myhost,MetricSource=sesame-seed`
	sdims := strings.Split(cwDimensions, ",")
	for _, sdim := range sdims {
		name := strings.Split(sdim, "=")[0]
		value := strings.Split(sdim, "=")[1]
		dim := cloudwatch.Dimension{
			Name:  &name,
			Value: &value,
		}
		dims = append(dims, &dim)
	}
	return dims
}

func functionCWPutMetric(region, cwNamespace, cwMetricName, cwDimensions, cwAssumeRoleArn string, cwValue float64) (err error) {
	// first handle the assume role business
	if cfg.verbosity {
		fmt.Println("Inside functionCWPutMetric")
	}
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)
	svcProfile := sts.New(sess)

	roleSessionName := "sesame-seed"
	var duration int64
	duration = 3200
	params := &sts.AssumeRoleInput{
		RoleArn:         &cwAssumeRoleArn,
		RoleSessionName: &roleSessionName,
		DurationSeconds: &duration,
	}
	// now try the assume-role with the loaded creds
	resp, err := svcProfile.AssumeRole(params)
	if err != nil {
		return err
	}

	if cfg.verbosity {
		fmt.Printf("Successfully assumed role %s", cwAssumeRoleArn)
	}

	statCreds := credentials.NewStaticCredentials(
		*resp.Credentials.AccessKeyId,
		*resp.Credentials.SecretAccessKey,
		*resp.Credentials.SessionToken)

	sess = session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Credentials: statCreds,
			Region:      &region,
		},
	}))

	if err != nil {
		handleErr(err)
		return err
	}
	c := cloudwatch.New(sess)
	dims := parseCWDimensions(cwDimensions)
	metricDatum := cloudwatch.MetricDatum{
		MetricName: &cwMetricName,
		Dimensions: dims,
		Value:      &cwValue,
	}
	var metricDatums []*cloudwatch.MetricDatum
	metricDatums = append(metricDatums, &metricDatum)
	cwInput := cloudwatch.PutMetricDataInput{
		Namespace:  &cwNamespace,
		MetricData: metricDatums,
	}
	if cfg.verbosity {
		fmt.Println("Attempting to put metric...")
	}
	_, err = c.PutMetricData(&cwInput)
	if err != nil {
		handleErr(err)
		return err
	}
	fmt.Printf("Successfully put %d metric(s) of value %f\n", len(metricDatums), cwValue)
	return err
}

func functionS3Download(region, s3bucket, s3key, s3dest string) (err error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)
	if err != nil {
		handleErr(err)
		return err
	}
	s := s3.New(sess)
	goInput := s3.GetObjectInput{
		Bucket: &s3bucket,
		Key:    &s3key,
	}
	result, err := s.GetObject(&goInput)
	if err != nil {
		handleErr(err)
		return err
	}
	body, err := ioutil.ReadAll(result.Body)
	if err != nil {
		handleErr(err)
		return err
	}
	f, err := os.Create(s3dest)
	if err != nil {
		handleErr(err)
		return err
	}
	w := bufio.NewWriter(f)
	written, err := w.Write(body)
	if err != nil {
		handleErr(err)
		return err
	}
	w.Flush()
	fmt.Printf("Wrote %d bytes to disk to filename '%s'.\n", written, s3dest)
	return err
}

func main() {
	var function, region string
	var s3bucket, s3key, s3dest string
	var cwNamespace, cwMetricName, cwDimensions, cwAssumeRoleArn string
	var cwValue float64
	var versionFlag, verbose bool
	flag.StringVar(&region, "region", "us-east-1", "Region to use for all actions. Set as 'instance' to pull the instance's region from metadata.")
	flag.StringVar(&function, "function", "s3download", "Which function to perform ('s3download' or 'cwputmetric'")
	// process s3 vars
	flag.StringVar(&s3bucket, "s3bucket", "my-bucket", "Name of bucket to get object from")
	flag.StringVar(&s3key, "s3key", "/path/to/my/object", "Full key path to object")
	flag.StringVar(&s3dest, "s3dest", "/path/on/disk", "path to destination on disk")
	// now process cloudwatch vars
	flag.StringVar(&cwNamespace, "cwnamespace", "my-namespace", "cloudwatch metric namespace")
	flag.Float64Var(&cwValue, "cwvalue", 42, "cloudwatch metric value must be convertable to float64, i.e, no strings")
	flag.StringVar(&cwMetricName, "cwmetricname", "my-metric", "cloudwatch metric name")
	flag.StringVar(&cwDimensions, "cwdimensions", "Host=myhost,MetricSource=sesame-seed", "cloudwatch metric dimensions. Key value pairs separated by comma. ")
	flag.StringVar(&cwAssumeRoleArn, "cwassumerolearn", "arn:aws:iam::123456789012:role/devopsdept/metrics-putter", "role to assume before attemping putmetric")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&versionFlag, "version", false, "Prints version and exits")
	flag.Parse()
	if versionFlag {
		fmt.Printf("sesame-seed %s\n", version)
		os.Exit(0)
	}
	cfg.verbosity = verbose
	if region == "instance" {
		svc := ec2metadata.New(session.New(), aws.NewConfig())
		id, err := svc.GetInstanceIdentityDocument()
		if err != nil {
			handleErr(err)
			panic(err)
		}
		region = id.Region
	}
	var err error

	if cfg.verbosity {
		fmt.Printf("Using region: '%s'\n", region)
	}
	switch function {
	case "s3download":
		if cfg.verbosity {
			fmt.Println("Attempting to run functionS3Download...")
		}
		err = functionS3Download(region, s3bucket, s3key, s3dest)
		if err != nil {
			handleErr(err)
			os.Exit(1)
		}
		os.Exit(0)
	case "cwputmetric":
		if cfg.verbosity {
			fmt.Println("Attempting to run functionCWPutMetric...")
		}
		err = functionCWPutMetric(region, cwNamespace, cwMetricName, cwDimensions, cwAssumeRoleArn, cwValue)
		if err != nil {
			handleErr(err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}
