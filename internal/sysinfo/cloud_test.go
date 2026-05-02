package sysinfo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCloudAWSDetection(t *testing.T) {
	awsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest/api/token":
			w.Write([]byte("TKN"))
		case "/latest/meta-data/instance-type":
			if r.Header.Get("X-aws-ec2-metadata-token") != "TKN" {
				http.Error(w, "no token", 401)
				return
			}
			w.Write([]byte("c6i.16xlarge"))
		case "/latest/meta-data/placement/availability-zone":
			w.Write([]byte("us-east-1a"))
		case "/latest/meta-data/placement/region":
			w.Write([]byte("us-east-1"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer awsSrv.Close()

	probes := []cloudProbe{newAWSProbeWithBase(awsSrv.URL)}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	got := runCloudProbes(ctx, probes, 500*time.Millisecond)
	if got == nil {
		t.Fatal("expected aws detected, got nil")
	}
	if got.Provider != "aws" {
		t.Errorf("provider = %q", got.Provider)
	}
	if got.InstanceType != "c6i.16xlarge" {
		t.Errorf("instance = %q", got.InstanceType)
	}
}

func TestCloudAllFailReturnsNil(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", 500)
	}))
	defer dead.Close()

	probes := []cloudProbe{newAWSProbeWithBase(dead.URL)}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	got := runCloudProbes(ctx, probes, 100*time.Millisecond)
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}
