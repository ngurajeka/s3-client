package download

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"s3-client/internal/s3uri"
	"s3-client/internal/shared/config"
	"s3-client/internal/shared/s3ops"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func newFlagSet() *flag.FlagSet {
	return flag.NewFlagSet("download", flag.ContinueOnError)
}

func printUsage(fs *flag.FlagSet) {
	fmt.Fprintln(os.Stderr, "Usage: s3-client download [flags] s3://bucket/key/path")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Examples:")
	fmt.Fprintln(os.Stderr, "  s3-client download s3://my-bucket/backups/file.tgz")
	fmt.Fprintln(os.Stderr, "  s3-client download -profile prod -region us-west-2 s3://my-bucket/data/dump.tar.gz")
	fmt.Fprintln(os.Stderr, "  s3-client download -chunk-size 25 -concurrency 8 -output /tmp/file.tgz s3://my-bucket/file.tgz")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Flags:")
	fs.PrintDefaults()
}

const defaultConcurrency = 5

const (
	stateWaiting = iota
	stateDownloading
	stateDone
	stateFailed
)

type downloader struct {
	client      *s3.Client
	bucket      string
	key         string
	outputPath  string
	chunkSize   int64
	concurrency int
}

type chunk struct {
	index int
	start int64
	end   int64
}

type progressBar struct {
	mu          sync.Mutex
	totalChunks int
	totalBytes  int64
	chunkStates []int32
	downloaded  *int64
	startTime   time.Time
	lastBytes   int64
	lastTime    time.Time
	speedMBs    float64
	rendered    bool
}

func newProgressBar(totalChunks int, totalBytes int64, downloaded *int64) *progressBar {
	return &progressBar{
		totalChunks: totalChunks,
		totalBytes:  totalBytes,
		chunkStates: make([]int32, totalChunks),
		downloaded:  downloaded,
		startTime:   time.Now(),
		lastTime:    time.Now(),
	}
}

func (p *progressBar) setState(chunkIdx int, state int32) {
	atomic.StoreInt32(&p.chunkStates[chunkIdx], state)
}

func (p *progressBar) render() {
	p.mu.Lock()
	defer p.mu.Unlock()

	bytes := atomic.LoadInt64(p.downloaded)
	now := time.Now()

	elapsed := now.Sub(p.lastTime).Seconds()
	if elapsed >= 0.5 {
		delta := bytes - p.lastBytes
		p.speedMBs = float64(delta) / elapsed / 1024 / 1024
		p.lastBytes = bytes
		p.lastTime = now
	}

	totalMB := float64(p.totalBytes) / 1024 / 1024
	doneMB := float64(bytes) / 1024 / 1024
	pct := doneMB / totalMB * 100
	if pct > 100 {
		pct = 100
	}

	etaStr := "—"
	if p.speedMBs > 0 && pct < 100 {
		remainMB := totalMB - doneMB
		etaSec := remainMB / p.speedMBs
		etaStr = formatDuration(time.Duration(etaSec * float64(time.Second)))
	}

	waiting, downloading, done, failed := 0, 0, 0, 0
	for i := range p.chunkStates {
		switch atomic.LoadInt32(&p.chunkStates[i]) {
		case stateWaiting:
			waiting++
		case stateDownloading:
			downloading++
		case stateDone:
			done++
		case stateFailed:
			failed++
		}
	}

	numLines := 5
	if p.rendered {
		fmt.Printf("\033[%dA", numLines)
		for i := 0; i < numLines; i++ {
			fmt.Print("\033[2K\n")
		}
		fmt.Printf("\033[%dA", numLines)
	}
	p.rendered = true

	barWidth := 50
	filled := int(float64(barWidth) * pct / 100)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	totalElapsed := time.Since(p.startTime)

	fmt.Printf("  Progress: %5.1f%%  [%s]  ETA: %s\n", pct, bar, etaStr)
	fmt.Printf("  %.2f / %.2f MB   speed: %.2f MB/s   elapsed: %s\n",
		doneMB, totalMB, p.speedMBs, formatDuration(totalElapsed))
	fmt.Printf("  Chunks ▸ total: %d   ⬜ waiting: %d   🔄 active: %d   ✅ done: %d",
		p.totalChunks, waiting, downloading, done)
	if failed > 0 {
		fmt.Printf("   ❌ failed: %d", failed)
	}
	fmt.Println()

	fmt.Printf("  Chunk map (▓=done  ▒=active  ░=waiting  ✗=failed):\n")
	fmt.Print("  [")
	for i := range p.chunkStates {
		switch atomic.LoadInt32(&p.chunkStates[i]) {
		case stateWaiting:
			fmt.Print("░")
		case stateDownloading:
			fmt.Print("\033[33m▒\033[0m")
		case stateDone:
			fmt.Print("\033[32m▓\033[0m")
		case stateFailed:
			fmt.Print("\033[31m✗\033[0m")
		}
	}
	fmt.Println("]")
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func Run(args []string) int {
	fs := newFlagSet()
	output := fs.String("output", "", "Output file path (defaults to basename of the S3 key)")
	chunkMB := fs.Int("chunk-size", 10, "Chunk size in MB")
	concurrency := fs.Int("concurrency", defaultConcurrency, "Number of parallel chunk downloads")

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

	bucket, key, err := s3uri.Parse(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	outputPath := *output
	if outputPath == "" {
		outputPath = filepath.Base(key)
	}

	ctx := context.Background()
	cfg, err := config.Load(ctx, *opts)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "\n❌ AWS credentials not found or invalid.")
		fmt.Fprintln(os.Stderr, "\nOptions to fix:")
		fmt.Fprintln(os.Stderr, "  1. s3-client download -profile myprofile s3://...")
		fmt.Fprintln(os.Stderr, "  2. export AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=...")
		fmt.Fprintln(os.Stderr, "  3. export AWS_PROFILE=myprofile")
		fmt.Fprintf(os.Stderr, "\nDetail: %v\n", err)
		return 1
	}
	if opts.Profile != "" {
		fmt.Printf("Using AWS profile: %s (source: %s)\n", opts.Profile, creds.Source)
	}

	client := s3.NewFromConfig(cfg)
	d := &downloader{
		client:      client,
		bucket:      bucket,
		key:         key,
		outputPath:  outputPath,
		chunkSize:   int64(*chunkMB) * 1024 * 1024,
		concurrency: *concurrency,
	}

	fmt.Printf("Downloading  s3://%s/%s\n", bucket, key)
	fmt.Printf("Output       %s\n", outputPath)
	fmt.Printf("Chunk size   %d MB  |  Concurrency: %d\n\n", *chunkMB, *concurrency)

	start := time.Now()
	if err := d.download(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ Download failed: %v\n", err)
		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "AccessDenied") {
			fmt.Fprintln(os.Stderr, "Tip: 403/AccessDenied — credentials lack s3:GetObject on this bucket/key.")
		} else if strings.Contains(err.Error(), "NoSuchKey") {
			fmt.Fprintf(os.Stderr, "Tip: key %q not found in bucket %q.\n", key, bucket)
		} else if strings.Contains(err.Error(), "400") {
			fmt.Fprintln(os.Stderr, "Tip: 400 Bad Request — bucket may be in a different region. Try -region <region>.")
		}
		return 1
	}

	elapsed := time.Since(start)
	info, _ := os.Stat(outputPath)
	sizeMB := float64(info.Size()) / 1024 / 1024
	fmt.Printf("\n✓ Done! %.2f MB in %s (avg %.2f MB/s)\n",
		sizeMB, formatDuration(elapsed), sizeMB/elapsed.Seconds())
	return 0
}

func (d *downloader) download(ctx context.Context) error {
	meta, err := s3ops.HeadObject(ctx, d.client, d.bucket, d.key)
	if err != nil {
		return fmt.Errorf("HeadObject failed: %w", err)
	}
	totalSize := meta.Size
	fmt.Printf("Object size: %.2f MB (%d bytes)\n", float64(totalSize)/1024/1024, totalSize)

	f, err := os.OpenFile(d.outputPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	if err := f.Truncate(totalSize); err != nil {
		return fmt.Errorf("failed to pre-allocate file: %w", err)
	}

	var chunks []chunk
	for i := int64(0); i < totalSize; i += d.chunkSize {
		end := i + d.chunkSize - 1
		if end >= totalSize {
			end = totalSize - 1
		}
		chunks = append(chunks, chunk{index: len(chunks), start: i, end: end})
	}
	totalChunks := len(chunks)
	fmt.Printf("Splitting into %d chunks\n\n", totalChunks)

	var downloaded int64
	pb := newProgressBar(totalChunks, totalSize, &downloaded)

	ticker := time.NewTicker(150 * time.Millisecond)
	stopProgress := make(chan struct{})
	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		for {
			select {
			case <-ticker.C:
				pb.render()
			case <-stopProgress:
				ticker.Stop()
				pb.render()
				return
			}
		}
	}()

	chunkCh := make(chan chunk, totalChunks)
	for _, c := range chunks {
		chunkCh <- c
	}
	close(chunkCh)

	errCh := make(chan error, d.concurrency)
	var wg sync.WaitGroup

	for i := 0; i < d.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range chunkCh {
				pb.setState(c.index, stateDownloading)

				data, err := s3ops.DownloadRange(ctx, d.client, d.bucket, d.key, s3ops.RangeDownload{
					Start: c.start,
					End:   c.end,
				})
				if err != nil {
					pb.setState(c.index, stateFailed)
					errCh <- fmt.Errorf("chunk %d (%d-%d) DownloadRange failed: %w", c.index, c.start, c.end, err)
					return
				}

				if _, err := f.WriteAt(data, c.start); err != nil {
					pb.setState(c.index, stateFailed)
					errCh <- fmt.Errorf("chunk %d write failed: %w", c.index, err)
					return
				}

				atomic.AddInt64(&downloaded, int64(len(data)))
				pb.setState(c.index, stateDone)
			}
		}()
	}

	wg.Wait()
	close(stopProgress)
	<-progressDone
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}
