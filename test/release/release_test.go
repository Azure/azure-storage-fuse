/*
Licensed under the MIT License <http://opensource.org/licenses/MIT>.
Copyright (c) 2026 Microsoft Corporation. All rights reserved.
*/

package release_test

import (
	"encoding/csv"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func releaseAssetScript(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../blobfuse2-release.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var pipeline struct {
		Stages []struct {
			Stage string
			Jobs  []struct {
				Steps []struct {
					DisplayName string `yaml:"displayName"`
					Script      string
				}
			}
		}
	}
	if err := yaml.Unmarshal(data, &pipeline); err != nil {
		t.Fatal(err)
	}
	for _, stage := range pipeline.Stages {
		if stage.Stage == "ReleaseArtifacts" {
			for _, job := range stage.Jobs {
				for _, step := range job.Steps {
					if step.DisplayName == "Select Ubuntu and Azure Linux Release Assets" {
						return step.Script
					}
				}
			}
		}
	}
	t.Fatal("release asset selection script not found")
	return ""
}

func TestReleaseAssets(t *testing.T) {
	script := releaseAssetScript(t)
	for _, missing := range []string{"", "Ubuntu", "x86_64", "aarch64"} {
		name := missing
		if name == "" {
			name = "all packages present"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			ubuntu := []string{
				"blobfuse2-2.5.6-preview.2-Ubuntu-18.04.x86_64.deb",
				"blobfuse2-2.5.6-preview.2-Ubuntu-20.04.x86_64.deb",
				"blobfuse2-2.5.6-preview.2-Ubuntu-22.04.arm64.deb",
				"blobfuse2-2.5.6-preview.2-Ubuntu-26.04.arm64.deb",
			}
			var expected []string
			files := []string{
				"blobfuse2/blobfuse2-2.5.6-preview.2-Debian-12.0.x86_64.deb",
				"blobfuse2/blobfuse2-2.5.6-preview.2-RHEL-9.0.x86_64.rpm",
				"blobfuse2/blobfuse2-2.5.6-preview.2-Rocky-9.0.x86_64.rpm",
				"blobfuse2/blobfuse2-2.5.6-preview.2-SUSE-15Gen2.x86_64.rpm",
				"blobfuse2/blobfuse2-2.5.6-preview.2-CentOS-8.0.x86_64.rpm",
			}
			if missing != "Ubuntu" {
				for _, file := range ubuntu {
					files = append(files, "blobfuse2/"+file)
					expected = append(expected, file)
				}
			}
			for _, arch := range []string{"x86_64", "aarch64"} {
				if missing != arch {
					files = append(files, "blobfuse2-signed/mariner/blobfuse2-2.5.6-preview.2-fuse3."+arch+".rpm")
					expected = append(expected, "blobfuse2-2.5.6-preview.2-AzureLinux-3.0."+arch+".rpm")
				}
			}
			for _, file := range files {
				path := filepath.Join(dir, file)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(file), 0644); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("bash", "-c", script)
			cmd.Dir = dir
			output, err := cmd.CombinedOutput()
			if missing != "" {
				if err == nil {
					t.Fatalf("expected missing %s packages to fail, got success:\n%s", missing, output)
				}
				return
			}
			if err != nil {
				t.Fatalf("asset selection failed: %v\n%s", err, output)
			}
			entries, err := os.ReadDir(filepath.Join(dir, "release-assets"))
			if err != nil {
				t.Fatal(err)
			}
			var actual []string
			for _, entry := range entries {
				actual = append(actual, entry.Name())
				source := "blobfuse2/" + entry.Name()
				if strings.Contains(entry.Name(), "-AzureLinux-3.0.") {
					source = "blobfuse2-signed/mariner/" + strings.Replace(entry.Name(), "-AzureLinux-3.0.", "-fuse3.", 1)
				}
				data, err := os.ReadFile(filepath.Join(dir, "release-assets", entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
				if string(data) != source {
					t.Fatalf("package content changed for %s", entry.Name())
				}
			}
			slices.Sort(expected)
			if !slices.Equal(expected, actual) {
				t.Fatalf("release assets = %v, want %v", actual, expected)
			}
		})
	}
}

func TestPackageFeedScope(t *testing.T) {
	file, err := os.Open("../../setup/packages.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comment = '#'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{
		"Ubuntu-18.04", "Ubuntu-20.04", "Ubuntu-22.04", "Ubuntu-22.04",
		"Ubuntu-24.04", "Ubuntu-24.04", "Ubuntu-26.04", "Ubuntu-26.04",
		"Mariner-3.0-x86_64", "Mariner-3.0-aarch64",
	}
	var actual []string
	for _, row := range rows {
		actual = append(actual, row[0])
	}
	if !slices.Equal(expected, actual) {
		t.Fatalf("package feed targets = %v, want %v", actual, expected)
	}
}
