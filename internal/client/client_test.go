package client

import (
	"testing"
)

const (
	jkuRemoteURL   = "https://jku.github.io/tuf-demo/metadata"
	expectedTarget = "file1.txt"
)

func TestNewClient_Remote(t *testing.T) {
	client, err := NewClient(jkuRemoteURL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	if client.metadataURL != jkuRemoteURL {
		t.Errorf("Expected metadataURL %s, got %s", jkuRemoteURL, client.metadataURL)
	}
}

func TestNewClient_Remote_AutoDetectHashPrefix(t *testing.T) {
	client, err := NewClient(jkuRemoteURL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Should auto-detect hash prefix from consistent_snapshot
	if client.metadataURL != jkuRemoteURL {
		t.Errorf("Expected metadataURL %s, got %s", jkuRemoteURL, client.metadataURL)
	}
}

func TestUpdate_Remote(t *testing.T) {
	client, err := NewClient(jkuRemoteURL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	err = client.Update()
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
}


func TestGetTargets_Remote(t *testing.T) {
	client, err := NewClient(jkuRemoteURL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	err = client.Update()
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	targets, err := client.GetTargets()
	if err != nil {
		t.Fatalf("GetTargets failed: %v", err)
	}

	if len(targets) == 0 {
		t.Error("Expected at least one target, got 0")
	}

	// Check if expected target exists
	found := false
	for _, target := range targets {
		if target.Name == expectedTarget {
			found = true
			if target.Length != 5 {
				t.Errorf("Expected target length 5, got %d", target.Length)
			}
			break
		}
	}

	if !found {
		t.Errorf("Expected target %s not found", expectedTarget)
	}
}


func TestGetRepositoryInfo_Remote(t *testing.T) {
	client, err := NewClient(jkuRemoteURL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	err = client.Update()
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	info, err := client.GetRepositoryInfo()
	if err != nil {
		t.Fatalf("GetRepositoryInfo failed: %v", err)
	}

	if info.RootVersion == 0 {
		t.Error("Expected root version > 0")
	}

	if info.MetadataURL != jkuRemoteURL {
		t.Errorf("Expected metadata URL %s, got %s", jkuRemoteURL, info.MetadataURL)
	}
}


func TestGetDelegations_Remote(t *testing.T) {
	client, err := NewClient(jkuRemoteURL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	err = client.Update()
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	delegations, err := client.GetDelegations()
	if err != nil {
		t.Fatalf("GetDelegations failed: %v", err)
	}

	// jku demo has delegations
	if len(delegations) == 0 {
		t.Error("Expected delegations, got 0")
	}

	// Check for known delegation
	found := false
	for _, del := range delegations {
		if del.Name == "jku" || del.Name == "rdimitrov" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find 'jku' or 'rdimitrov' delegation")
	}
}

