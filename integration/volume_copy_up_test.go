/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integration

import (
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

func TestVolumeCopyUp(t *testing.T) {
	const (
		testImage   = "gcr.io/k8s-cri-containerd/volume-copy-up:1.0"
		execTimeout = time.Minute
	)

	t.Logf("Create a sandbox")
	sbConfig := PodSandboxConfig("sandbox", "volume-copy-up")
	sb, err := runtimeService.RunPodSandbox(sbConfig)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, runtimeService.StopPodSandbox(sb))
		assert.NoError(t, runtimeService.RemovePodSandbox(sb))
	}()

	t.Logf("Pull test image")
	_, err = imageService.PullImage(&runtime.ImageSpec{Image: testImage}, nil)
	require.NoError(t, err)

	t.Logf("Create a container with volume-copy-up test image")
	cnConfig := ContainerConfig(
		"container",
		testImage,
		WithCommand("tail", "-f", "/dev/null"),
	)
	cn, err := runtimeService.CreateContainer(sb, cnConfig, sbConfig)
	require.NoError(t, err)

	t.Logf("Start the container")
	require.NoError(t, runtimeService.StartContainer(cn))

	// gcr.io/k8s-cri-containerd/volume-copy-up:1.0 contains a test_dir
	// volume, which contains a test_file with content "test_content".
	t.Logf("Check whether volume contains the test file")
	stdout, stderr, err := runtimeService.ExecSync(cn, []string{
		"cat",
		"/test_dir/test_file",
	}, execTimeout)
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Equal(t, "test_content\n", string(stdout))

	t.Logf("Check host path of the volume")
	hostCmd := fmt.Sprintf("ls %s/containers/%s/volumes/*/test_file | xargs cat", criContainerdRoot, cn)
	output, err := exec.Command("sh", "-c", hostCmd).CombinedOutput()
	require.NoError(t, err)
	assert.Equal(t, "test_content\n", string(output))

	t.Logf("Update volume from inside the container")
	_, _, err = runtimeService.ExecSync(cn, []string{
		"sh",
		"-c",
		"echo new_content > /test_dir/test_file",
	}, execTimeout)
	require.NoError(t, err)

	t.Logf("Check whether host path of the volume is updated")
	output, err = exec.Command("sh", "-c", hostCmd).CombinedOutput()
	require.NoError(t, err)
	assert.Equal(t, "new_content\n", string(output))
}
