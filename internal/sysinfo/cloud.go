package sysinfo

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/kai-w/bscbench/internal/report"
)

const probeTimeout = 500 * time.Millisecond

type cloudProbe interface {
	Detect(ctx context.Context) *report.CloudInfo
	Provider() string
}

func collectCloud(ctx context.Context) *report.CloudInfo {
	probes := []cloudProbe{
		newAWSProbeWithBase("http://169.254.169.254"),
		newAliyunProbeWithBase("http://100.100.100.200"),
		newGCPProbeWithBase("http://metadata.google.internal"),
	}
	return runCloudProbes(ctx, probes, probeTimeout)
}

// runCloudProbes runs probes in parallel, returns the first non-nil result.
func runCloudProbes(ctx context.Context, probes []cloudProbe, timeout time.Duration) *report.CloudInfo {
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	results := make(chan *report.CloudInfo, len(probes))
	var wg sync.WaitGroup
	for _, p := range probes {
		wg.Add(1)
		go func(p cloudProbe) {
			defer wg.Done()
			results <- p.Detect(probeCtx)
		}(p)
	}
	go func() { wg.Wait(); close(results) }()

	for r := range results {
		if r != nil {
			return r
		}
	}
	return nil
}

// --- AWS IMDSv2 ---

type awsProbe struct {
	base string
	hc   *http.Client
}

func newAWSProbeWithBase(base string) *awsProbe {
	return &awsProbe{base: base, hc: &http.Client{Timeout: probeTimeout}}
}

func (p *awsProbe) Provider() string { return "aws" }

func (p *awsProbe) Detect(ctx context.Context) *report.CloudInfo {
	tokenReq, _ := http.NewRequestWithContext(ctx, "PUT", p.base+"/latest/api/token", nil)
	tokenReq.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "30")
	tokenResp, err := p.hc.Do(tokenReq)
	if err != nil || tokenResp.StatusCode != 200 {
		if tokenResp != nil {
			tokenResp.Body.Close()
		}
		return nil
	}
	tokenB, _ := io.ReadAll(tokenResp.Body)
	tokenResp.Body.Close()
	token := string(tokenB)

	get := func(path string) string {
		req, _ := http.NewRequestWithContext(ctx, "GET", p.base+path, nil)
		req.Header.Set("X-aws-ec2-metadata-token", token)
		resp, err := p.hc.Do(req)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			return ""
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}

	inst := get("/latest/meta-data/instance-type")
	if inst == "" {
		return nil
	}
	return &report.CloudInfo{
		Provider:     "aws",
		InstanceType: inst,
		AZ:           get("/latest/meta-data/placement/availability-zone"),
		Region:       get("/latest/meta-data/placement/region"),
	}
}

// --- Aliyun ---

type aliyunProbe struct {
	base string
	hc   *http.Client
}

func newAliyunProbeWithBase(base string) *aliyunProbe {
	return &aliyunProbe{base: base, hc: &http.Client{Timeout: probeTimeout}}
}

func (p *aliyunProbe) Provider() string { return "aliyun" }

func (p *aliyunProbe) Detect(ctx context.Context) *report.CloudInfo {
	get := func(path string) string {
		req, _ := http.NewRequestWithContext(ctx, "GET", p.base+path, nil)
		resp, err := p.hc.Do(req)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			return ""
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	inst := get("/latest/meta-data/instance/instance-type")
	if inst == "" {
		return nil
	}
	return &report.CloudInfo{
		Provider:     "aliyun",
		InstanceType: inst,
		AZ:           get("/latest/meta-data/zone-id"),
		Region:       get("/latest/meta-data/region-id"),
	}
}

// --- GCP ---

type gcpProbe struct {
	base string
	hc   *http.Client
}

func newGCPProbeWithBase(base string) *gcpProbe {
	return &gcpProbe{base: base, hc: &http.Client{Timeout: probeTimeout}}
}

func (p *gcpProbe) Provider() string { return "gcp" }

func (p *gcpProbe) Detect(ctx context.Context) *report.CloudInfo {
	get := func(path string) string {
		req, _ := http.NewRequestWithContext(ctx, "GET", p.base+path, nil)
		req.Header.Set("Metadata-Flavor", "Google")
		resp, err := p.hc.Do(req)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			return ""
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	inst := get("/computeMetadata/v1/instance/machine-type")
	if inst == "" {
		return nil
	}
	return &report.CloudInfo{
		Provider:     "gcp",
		InstanceType: inst,
		AZ:           get("/computeMetadata/v1/instance/zone"),
		Region:       "",
	}
}
