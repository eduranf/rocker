/*-
 * Copyright 2015 Grammarly, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package build2

import (
	"testing"

	"github.com/kr/pretty"
	"github.com/stretchr/testify/mock"

	"github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
)

// =========== Testing FROM ===========

func TestCommandFrom_Existing(t *testing.T) {
	b, c := makeBuild(t, "", Config{})
	cmd := &CommandFrom{ConfigCommand{
		args: []string{"existing"},
	}}

	img := &docker.Image{
		ID: "123",
		Config: &docker.Config{
			Hostname: "localhost",
		},
	}

	c.On("InspectImage", "existing").Return(img, nil).Once()

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	c.AssertExpectations(t)
	assert.Equal(t, "123", state.imageID)
	assert.Equal(t, "localhost", state.config.Hostname)
}

func TestCommandFrom_PullExisting(t *testing.T) {
	b, c := makeBuild(t, "", Config{Pull: true})
	cmd := &CommandFrom{ConfigCommand{
		args: []string{"existing"},
	}}

	img := &docker.Image{
		ID: "123",
		Config: &docker.Config{
			Hostname: "localhost",
		},
	}

	c.On("PullImage", "existing").Return(nil).Once()
	c.On("InspectImage", "existing").Return(img, nil).Once()

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	c.AssertExpectations(t)
	assert.Equal(t, "123", state.imageID)
	assert.Equal(t, "localhost", state.config.Hostname)
}

func TestCommandFrom_NotExisting(t *testing.T) {
	b, c := makeBuild(t, "", Config{})
	cmd := &CommandFrom{ConfigCommand{
		args: []string{"not-existing"},
	}}

	var nilImg *docker.Image

	img := &docker.Image{
		ID:     "123",
		Config: &docker.Config{},
	}

	c.On("InspectImage", "not-existing").Return(nilImg, nil).Once()
	c.On("PullImage", "not-existing").Return(nil).Once()
	c.On("InspectImage", "not-existing").Return(img, nil).Once()

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	c.AssertExpectations(t)
	assert.Equal(t, "123", state.imageID)
}

func TestCommandFrom_AfterPullNotExisting(t *testing.T) {
	b, c := makeBuild(t, "", Config{})
	cmd := &CommandFrom{ConfigCommand{
		args: []string{"not-existing"},
	}}

	var nilImg *docker.Image

	c.On("InspectImage", "not-existing").Return(nilImg, nil).Twice()
	c.On("PullImage", "not-existing").Return(nil).Once()

	_, err := cmd.Execute(b)
	c.AssertExpectations(t)
	assert.Equal(t, "FROM: Failed to inspect image after pull: not-existing", err.Error())
}

// =========== Testing RUN ===========

func TestCommandRun_Simple(t *testing.T) {
	b, c := makeBuild(t, "", Config{})
	cmd := &CommandRun{ConfigCommand{
		args: []string{"whoami"},
	}}

	origCmd := []string{"/bin/program"}
	b.state.config.Cmd = origCmd
	b.state.imageID = "123"

	c.On("CreateContainer", mock.AnythingOfType("State")).Return("456", nil).Run(func(args mock.Arguments) {
		arg := args.Get(0).(State)
		assert.Equal(t, []string{"/bin/sh", "-c", "whoami"}, arg.config.Cmd)
	}).Once()

	c.On("RunContainer", "456", false).Return(nil).Once()

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	c.AssertExpectations(t)
	assert.Equal(t, origCmd, b.state.config.Cmd)
	assert.Equal(t, origCmd, state.config.Cmd)
	assert.Equal(t, "123", state.imageID)
	assert.Equal(t, "456", state.containerID)
}

// =========== Testing COMMIT ===========

func TestCommandCommit_Simple(t *testing.T) {
	b, c := makeBuild(t, "", Config{})
	cmd := &CommandCommit{}

	origCommitMsg := []string{"a", "b"}
	b.state.containerID = "456"
	b.state.commitMsg = origCommitMsg

	c.On("CommitContainer", mock.AnythingOfType("State"), "a; b").Return("789", nil).Once()
	c.On("RemoveContainer", "456").Return(nil).Once()

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	c.AssertExpectations(t)
	assert.Equal(t, origCommitMsg, b.state.commitMsg)
	assert.Equal(t, []string{}, state.commitMsg)
	assert.Equal(t, []string(nil), state.config.Cmd)
	assert.Equal(t, "789", state.imageID)
	assert.Equal(t, "", state.containerID)
}

func TestCommandCommit_NoContainer(t *testing.T) {
	b, c := makeBuild(t, "", Config{})
	cmd := &CommandCommit{}

	origCommitMsg := []string{"a", "b"}
	b.state.commitMsg = origCommitMsg

	c.On("CreateContainer", mock.AnythingOfType("State")).Return("456", nil).Run(func(args mock.Arguments) {
		arg := args.Get(0).(State)
		assert.Equal(t, []string{"/bin/sh", "-c", "#(nop) a; b"}, arg.config.Cmd)
	}).Once()

	c.On("CommitContainer", mock.AnythingOfType("State"), "a; b").Return("789", nil).Once()
	c.On("RemoveContainer", "456").Return(nil).Once()

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	c.AssertExpectations(t)
	assert.Equal(t, origCommitMsg, b.state.commitMsg)
	assert.Equal(t, []string{}, state.commitMsg)
	assert.Equal(t, "789", state.imageID)
	assert.Equal(t, "", state.containerID)
}

func TestCommandCommit_NoCommitMsgs(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandCommit{}

	_, err := cmd.Execute(b)
	assert.Contains(t, err.Error(), "Nothing to commit")
}

// =========== Testing ENV ===========

func TestCommandEnv_Simple(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandEnv{ConfigCommand{
		args: []string{"type", "web", "env", "prod"},
	}}

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []string{"ENV type=web env=prod"}, state.commitMsg)
	assert.Equal(t, []string{"type=web", "env=prod"}, state.config.Env)
}

func TestCommandEnv_Advanced(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandEnv{ConfigCommand{
		args: []string{"type", "web", "env", "prod"},
	}}

	b.state.config.Env = []string{"env=dev", "version=1.2.3"}

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []string{"ENV type=web env=prod"}, state.commitMsg)
	assert.Equal(t, []string{"env=prod", "version=1.2.3", "type=web"}, state.config.Env)
}

// =========== Testing CMD ===========

func TestCommandCmd_Simple(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandCmd{ConfigCommand{
		args: []string{"apt-get", "install"},
	}}

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []string{"/bin/sh", "-c", "apt-get install"}, state.config.Cmd)
}

func TestCommandCmd_Json(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandCmd{ConfigCommand{
		args:  []string{"apt-get", "install"},
		attrs: map[string]bool{"json": true},
	}}

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []string{"apt-get", "install"}, state.config.Cmd)
}

// =========== Testing COPY ===========

func TestCommandCopy_Simple(t *testing.T) {
	// TODO: do we need to check the dest is always a directory?
	b, c := makeBuild(t, "", Config{})
	cmd := &CommandCopy{ConfigCommand{
		args: []string{"testdata/file.txt", "/file.txt"},
	}}

	c.On("CreateContainer", mock.AnythingOfType("State")).Return("456", nil).Run(func(args mock.Arguments) {
		arg := args.Get(0).(State)
		// TODO: a better check
		assert.True(t, len(arg.config.Cmd) > 0)
	}).Once()

	c.On("UploadToContainer", "456", mock.AnythingOfType("*io.PipeReader"), "/file.txt").Return(nil).Once()

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	// assert.Equal(t, []string{"/bin/sh", "-c", "apt-get install"}, state.config.Cmd)
	pretty.Println(state)

	c.AssertExpectations(t)
	assert.Equal(t, "456", state.containerID)
}
