package isogen

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"opendev.org/airship/airshipctl/pkg/config"
	"opendev.org/airship/airshipctl/pkg/document"
	"opendev.org/airship/airshipctl/pkg/log"
	"opendev.org/airship/airshipctl/testutil"
)

type mockContainer struct {
	imagePull        func() error
	runCommand       func() error
	runCommandOutput func() (io.ReadCloser, error)
	rmContainer      func() error
	getId            func() string
}

func (mc *mockContainer) ImagePull() error {
	return mc.imagePull()
}

func (mc *mockContainer) RunCommand([]string, io.Reader, []string, []string, bool) error {
	return mc.runCommand()
}

func (mc *mockContainer) RunCommandOutput([]string, io.Reader, []string, []string) (io.ReadCloser, error) {
	return mc.runCommandOutput()
}

func (mc *mockContainer) RmContainer() error {
	return mc.rmContainer()
}

func (mc *mockContainer) GetId() string {
	return mc.getId()
}

func TestBootstrapIso(t *testing.T) {
	fSys := testutil.SetupTestFs(t, "testdata")
	bundle, err := document.NewBundle(fSys, "/", "/")
	require.NoError(t, err, "Building Bundle Failed")

	tempVol, cleanup := testutil.TempDir(t, "bootstrap-test")
	defer cleanup(t)

	volBind := tempVol + ":/dst"
	testErr := fmt.Errorf("TestErr")
	testCfg := &config.Bootstrap{
		Container: &config.Container{
			Volume:           volBind,
			ContainerRuntime: "docker",
		},
		Builder: &config.Builder{
			UserDataFileName:      "user-data",
			NetworkConfigFileName: "net-conf",
		},
	}
	expOut := []string{
		"Creating cloud-init for ephemeral K8s",
		fmt.Sprintf("Running default container command. Mounted dir: [%s]", volBind),
		"ISO successfully built.",
		"Debug flag is set. Container TESTID stopped but not deleted.",
		"Removing container.",
	}

	tests := []struct {
		builder     *mockContainer
		cfg         *config.Bootstrap
		debug       bool
		expectedOut []string
		expectedErr error
	}{
		{
			builder: &mockContainer{
				runCommand: func() error { return testErr },
			},
			cfg:         testCfg,
			debug:       false,
			expectedOut: []string{expOut[0], expOut[1]},
			expectedErr: testErr,
		},
		{
			builder: &mockContainer{
				runCommand: func() error { return nil },
				getId:      func() string { return "TESTID" },
			},
			cfg:         testCfg,
			debug:       true,
			expectedOut: []string{expOut[0], expOut[1], expOut[2], expOut[3]},
			expectedErr: nil,
		},
		{
			builder: &mockContainer{
				runCommand:  func() error { return nil },
				getId:       func() string { return "TESTID" },
				rmContainer: func() error { return testErr },
			},
			cfg:         testCfg,
			debug:       false,
			expectedOut: []string{expOut[0], expOut[1], expOut[2], expOut[4]},
			expectedErr: testErr,
		},
	}

	for _, tt := range tests {
		outBuf := &bytes.Buffer{}
		log.Init(tt.debug, outBuf)
		actualErr := generateBootstrapIso(bundle, tt.builder, tt.cfg, tt.debug)
		actualOut := outBuf.String()

		for _, line := range tt.expectedOut {
			assert.True(t, strings.Contains(actualOut, line))
		}

		assert.Equal(t, tt.expectedErr, actualErr)
	}
}

func TestVerifyInputs(t *testing.T) {
	tempVol, cleanup := testutil.TempDir(t, "bootstrap-test")
	defer cleanup(t)

	tests := []struct {
		cfg         *config.Bootstrap
		args        []string
		expectedErr error
	}{
		{
			cfg: &config.Bootstrap{
				Container: &config.Container{},
			},
			expectedErr: config.ErrWrongConfig{},
		},
		{
			cfg: &config.Bootstrap{
				Container: &config.Container{
					Volume: tempVol + ":/dst",
				},
				Builder: &config.Builder{},
			},
			expectedErr: config.ErrWrongConfig{},
		},
		{
			cfg: &config.Bootstrap{
				Container: &config.Container{
					Volume: tempVol,
				},
				Builder: &config.Builder{
					UserDataFileName:      "user-data",
					NetworkConfigFileName: "net-conf",
				},
			},
			expectedErr: nil,
		},
		{
			cfg: &config.Bootstrap{
				Container: &config.Container{
					Volume: tempVol + ":/dst:/dst1",
				},
				Builder: &config.Builder{
					UserDataFileName:      "user-data",
					NetworkConfigFileName: "net-conf",
				},
			},
			expectedErr: config.ErrWrongConfig{},
		},
	}

	for _, tt := range tests {
		actualErr := verifyInputs(tt.cfg)
		assert.Equal(t, tt.expectedErr, actualErr)
	}
}
