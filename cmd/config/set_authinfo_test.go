/*
Copyright 2017 The Kubernetes Authors.

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

package config

import (
	"bytes"
	"fmt"
	"testing"

	kubeconfig "k8s.io/client-go/tools/clientcmd/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"opendev.org/airship/airshipctl/pkg/config"
	"opendev.org/airship/airshipctl/pkg/environment"
	"opendev.org/airship/airshipctl/testutil"
)

const (
	testUsername = "admin@kubernetes"
	testPassword = "adminPassword"
	testNewname  = "dummy"
	testOldname  = "def-user"
	pwdDelta     = "_changed"
)

type setAuthInfoTest struct {
	description    string
	givenConfig    *config.Config
	args           []string
	flags          []string
	expectedOutput string
	expectedConfig *config.Config
}

func TestConfigSetAuthInfo(t *testing.T) {
	cmdTests := []*testutil.CmdTest{
		{
			Name:    "config-cmd-set-authinfo-with-help",
			CmdLine: "--help",
			Cmd:     NewCmdConfigSetAuthInfo(nil),
		},
		{
			Name:    "config-cmd-set-authinfo-too-many-args",
			CmdLine: "arg1 arg2",
			Cmd:     NewCmdConfigSetAuthInfo(nil),
			Error:   fmt.Errorf("accepts %d arg(s), received %d", 1, 2),
		},
		{
			Name:    "config-cmd-set-authinfo-too-few-args",
			CmdLine: "",
			Cmd:     NewCmdConfigSetAuthInfo(nil),
			Error:   fmt.Errorf("accepts %d arg(s), received %d", 1, 0),
		},
	}

	for _, tt := range cmdTests {
		testutil.RunTest(t, tt)
	}
}

// initConfig creates an input config and an associated expected config
// Each of these config objects are associated with real files. Those files can be
// cleaned up by calling cleanup
func initConfig(t *testing.T, withUser bool, testname string) (
	given, expected *config.Config, cleanup func(*testing.T)) {
	given, givenCleanup := config.InitConfig(t)
	if withUser {
		kAuthInfo := kubeconfig.NewAuthInfo()
		kAuthInfo.Username = testUsername
		kAuthInfo.Password = testPassword
		given.KubeConfig().AuthInfos[testname] = kAuthInfo
		given.AuthInfos[testname].SetKubeAuthInfo(kAuthInfo)
	}

	expected, expectedCleanup := config.InitConfig(t)
	expected.AuthInfos[testname] = config.NewAuthInfo()

	expkAuthInfo := kubeconfig.NewAuthInfo()
	expkAuthInfo.Username = testUsername
	expkAuthInfo.Password = testPassword
	expected.KubeConfig().AuthInfos[testname] = expkAuthInfo
	expected.AuthInfos[testname].SetKubeAuthInfo(expkAuthInfo)

	return given, expected, func(tt *testing.T) {
		givenCleanup(tt)
		expectedCleanup(tt)
	}
}

func TestSetAuthInfo(t *testing.T) {
	given, expected, cleanup := initConfig(t, false, testNewname)
	defer cleanup(t)

	test := setAuthInfoTest{
		description: "Testing 'airshipctl config set-credential' with a new user",
		givenConfig: given,
		args:        []string{testNewname},
		flags: []string{
			"--" + config.FlagUsername + "=" + testUsername,
			"--" + config.FlagPassword + "=" + testPassword,
		},
		expectedOutput: fmt.Sprintf("User information %q created.\n", testNewname),
		expectedConfig: expected,
	}
	test.run(t)
}

func TestModifyAuthInfo(t *testing.T) {
	given, expected, cleanup := initConfig(t, true, testOldname)
	defer cleanup(t)

	expected.AuthInfos[testOldname].KubeAuthInfo().Password = testPassword + pwdDelta

	test := setAuthInfoTest{
		description: "Testing 'airshipctl config set-credential' with an existing user",
		givenConfig: given,
		args:        []string{testOldname},
		flags: []string{
			"--" + config.FlagPassword + "=" + testPassword + pwdDelta,
		},
		expectedOutput: fmt.Sprintf("User information %q modified.\n", testOldname),
		expectedConfig: expected,
	}
	test.run(t)
}

func (test setAuthInfoTest) run(t *testing.T) {
	// Get the Environment
	settings := &environment.AirshipCTLSettings{}
	settings.SetConfig(test.givenConfig)

	buf := bytes.NewBuffer([]byte{})

	cmd := NewCmdConfigSetAuthInfo(settings)
	cmd.SetOut(buf)
	cmd.SetArgs(test.args)
	err := cmd.Flags().Parse(test.flags)
	require.NoErrorf(t, err, "unexpected error flags args to command: %v,  flags: %v", err, test.flags)

	// Execute the Command
	// Which should Persist the File
	err = cmd.Execute()
	require.NoErrorf(t, err, "unexpected error executing command: %v, args: %v, flags: %v", err, test.args, test.flags)

	afterRunConf := settings.Config()

	// Find the AuthInfo Created or Modified
	afterRunAuthInfo, err := afterRunConf.GetAuthInfo(test.args[0])
	require.NoError(t, err)
	require.NotNil(t, afterRunAuthInfo)

	afterKauthinfo := afterRunAuthInfo.KubeAuthInfo()
	require.NotNil(t, afterKauthinfo)

	testKauthinfo := test.expectedConfig.KubeConfig().AuthInfos[test.args[0]]
	require.NotNil(t, testKauthinfo)

	assert.EqualValues(t, testKauthinfo.Username, afterKauthinfo.Username)
	assert.EqualValues(t, testKauthinfo.Password, afterKauthinfo.Password)

	// Test that the Return Message looks correct
	if len(test.expectedOutput) != 0 {
		assert.EqualValues(t, test.expectedOutput, buf.String())
	}
}
