package setcors

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"s3-client/internal/s3uri"
	"s3-client/internal/shared/config"
	"s3-client/internal/shared/s3ops"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func newFlagSet() *flag.FlagSet {
	return flag.NewFlagSet("set-cors", flag.ContinueOnError)
}

func printUsage(fs *flag.FlagSet) {
	fmt.Fprintln(os.Stderr, "Usage: s3-client set-cors [flags] s3://bucket")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Configure CORS (Cross-Origin Resource Sharing) for an S3 bucket.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Examples:")
	fmt.Fprintln(os.Stderr, "  s3-client set-cors s3://my-bucket -cors-file cors.json")
	fmt.Fprintln(os.Stderr, "  s3-client set-cors s3://my-bucket -cors-json '[{\"AllowedOrigins\":[\"*\"],\"AllowedMethods\":[\"GET\"]}]'")
	fmt.Fprintln(os.Stderr, "  s3-client set-cors s3://my-bucket -delete")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Flags:")
	fs.PrintDefaults()
}

func Run(args []string) int {
	fs := newFlagSet()
	corsFile := fs.String("cors-file", "", "Path to CORS configuration file (JSON)")
	corsJSON := fs.String("cors-json", "", "CORS configuration as JSON string")
	delete := fs.Bool("delete", false, "Delete CORS configuration")
	show := fs.Bool("show", false, "Show current CORS configuration")

	opts := &config.Options{}
	config.AddFlags(fs, opts)

	fs.Usage = func() {
		printUsage(fs)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return 1
	}

	s3URI := fs.Arg(0)
	bucket, _, err := s3uri.Parse(s3URI)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	ctx := context.Background()
	cfg, err := config.Load(ctx, *opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load AWS config: %v\n", err)
		return 1
	}

	client := s3.NewFromConfig(cfg)

	if *show {
		rules, err := s3ops.GetBucketCors(ctx, client, bucket)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		if rules == nil {
			fmt.Println("No CORS configuration set.")
			return 0
		}
		data, err := json.MarshalIndent(rules, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling: %v\n", err)
			return 1
		}
		fmt.Println(string(data))
		return 0
	}

	if *delete {
		err := s3ops.DeleteBucketCors(ctx, client, bucket)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Printf("CORS configuration deleted for bucket %s\n", bucket)
		return 0
	}

	var rules []s3ops.CORSRule

	if *corsFile != "" {
		data, err := os.ReadFile(*corsFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading CORS file: %v\n", err)
			return 1
		}
		rules, err = s3ops.ParseCORSConfig(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing CORS file: %v\n", err)
			return 1
		}
	} else if *corsJSON != "" {
		rules, err = s3ops.ParseCORSConfig([]byte(*corsJSON))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing CORS JSON: %v\n", err)
			return 1
		}
	} else {
		fmt.Fprintln(os.Stderr, "Error: Must specify either -cors-file, -cors-json, or -delete")
		fs.Usage()
		return 1
	}

	err = s3ops.PutBucketCors(ctx, client, bucket, rules)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Printf("CORS configuration set for bucket %s\n", bucket)
	return 0
}
