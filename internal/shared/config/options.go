package config

import "flag"

type Options struct {
	Region   string
	Profile  string
	Endpoint string
}

func AddFlags(fs *flag.FlagSet, opts *Options) {
	fs.StringVar(&opts.Region, "region", "", "AWS region (overrides env/config)")
	fs.StringVar(&opts.Profile, "profile", "", "AWS credentials/config profile name")
	fs.StringVar(&opts.Endpoint, "endpoint", "", "S3-compatible endpoint URL (e.g., http://localhost:9000)")
}

func (o *Options) IsEmpty() bool {
	return o.Region == "" && o.Profile == "" && o.Endpoint == ""
}
