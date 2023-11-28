//go:build mqdev
// +build mqdev

/*
© Copyright IBM Corporation 2018, 2023

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
package main

import (
	"crypto/tls"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ce "github.com/ibm-messaging/mq-container/test/container/containerengine"
)

// TestDevGoldenPath tests using the default values for the default developer config.
// Note: This test requires a separate container image to be available for the JMS tests.
func TestDevGoldenPath(t *testing.T) {
	t.Parallel()
	cli := ce.NewContainerClient()
	qm := "qm1"
	containerConfig := ce.ContainerConfig{
		Env: []string{
			"LICENSE=accept",
			"MQ_QMGR_NAME=" + qm,
			"DEBUG=true",
		},
	}
	id := runContainerWithPorts(t, cli, &containerConfig, []int{9443, 1414})
	defer cleanContainer(t, cli, id)
	waitForReady(t, cli, id)
	waitForWebReady(t, cli, id, insecureTLSConfig)
	t.Run("JMS", func(t *testing.T) {
		// Run the JMS tests, with no password specified.
		// Use OpenJDK JRE for running testing, pass false for 7th parameter.
		// Last parameter is blank as the test doesn't use TLS.
		runJMSTests(t, cli, id, false, "app", defaultAppPasswordOS, "false", "")
	})
	t.Run("REST admin", func(t *testing.T) {
		testRESTAdmin(t, cli, id, insecureTLSConfig, "")
	})
	t.Run("REST messaging", func(t *testing.T) {
		testRESTMessaging(t, cli, id, insecureTLSConfig, qm, "app", defaultAppPasswordWeb, "")
	})
	// Stop the container cleanly
	stopContainer(t, cli, id)
}

// TestDevSecure tests the default developer config using the a custom TLS key store and password.
// Note: This test requires a separate container image to be available for the JMS tests
func TestDevSecure(t *testing.T) {
	t.Parallel()

	cli := ce.NewContainerClient()

	const tlsPassPhrase string = "passw0rd"
	qm := "qm1"
	appPassword := "differentPassw0rd"
	containerConfig := ce.ContainerConfig{
		Env: []string{
			"LICENSE=accept",
			"MQ_QMGR_NAME=" + qm,
			"MQ_APP_PASSWORD=" + appPassword,
			"DEBUG=1",
			"WLP_LOGGING_MESSAGE_FORMAT=JSON",
			"MQ_ENABLE_EMBEDDED_WEB_SERVER_LOG=true",
		},
		Image: imageName(),
	}
	hostConfig := ce.ContainerHostConfig{
		Binds: []string{
			coverageBind(t),
			tlsDir(t, false) + ":/etc/mqm/pki/keys/default",
		},
	}
	// Assign a random port for the web server on the host
	// TODO: Don't do this for all tests
	var binding ce.PortBinding
	ports := []int{9443, 1414}
	for _, p := range ports {
		port := fmt.Sprintf("%v/tcp", p)
		binding = ce.PortBinding{
			ContainerPort: port,
			HostIP:        "0.0.0.0",
		}
		hostConfig.PortBindings = append(hostConfig.PortBindings, binding)
	}
	networkingConfig := ce.ContainerNetworkSettings{}
	ID, err := cli.ContainerCreate(&containerConfig, &hostConfig, &networkingConfig, t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanContainer(t, cli, ID)
	startContainer(t, cli, ID)
	waitForReady(t, cli, ID)
	cert := filepath.Join(tlsDir(t, true), "server.crt")
	waitForWebReady(t, cli, ID, createTLSConfig(t, cert, tlsPassPhrase))

	t.Run("JMS", func(t *testing.T) {
		// OpenJDK is used for running tests, hence pass "false" for 7th parameter.
		// Cipher name specified is compliant with non-IBM JRE naming.
		runJMSTests(t, cli, ID, true, "app", appPassword, "false", "TLS_RSA_WITH_AES_256_CBC_SHA256")
	})
	t.Run("REST admin", func(t *testing.T) {
		testRESTAdmin(t, cli, ID, insecureTLSConfig, "")
	})
	t.Run("REST messaging", func(t *testing.T) {
		testRESTMessaging(t, cli, ID, insecureTLSConfig, qm, "app", appPassword, "")
	})

	// Stop the container cleanly
	stopContainer(t, cli, ID)
}

func TestDevWebDisabled(t *testing.T) {
	t.Parallel()

	cli := ce.NewContainerClient()
	containerConfig := ce.ContainerConfig{
		Env: []string{
			"LICENSE=accept",
			"MQ_QMGR_NAME=qm1",
			"MQ_ENABLE_EMBEDDED_WEB_SERVER=false",
		},
	}
	id := runContainerWithPorts(t, cli, &containerConfig, []int{1414})
	defer cleanContainer(t, cli, id)
	waitForReady(t, cli, id)
	t.Run("Web", func(t *testing.T) {
		_, dspmqweb := cli.ExecContainer(id, "", []string{"dspmqweb"})
		if !strings.Contains(dspmqweb, "Server mqweb is not running.") && !strings.Contains(dspmqweb, "MQWB1125I") {
			t.Errorf("Expected dspmqweb to say 'Server is not running' or 'MQWB1125I'; got \"%v\"", dspmqweb)
		}
	})
	t.Run("JMS", func(t *testing.T) {
		// Run the JMS tests, with no password specified
		// OpenJDK is used for running tests, hence pass "false" for 7th parameter.
		// Last parameter is blank as the test doesn't use TLS.
		runJMSTests(t, cli, id, false, "app", defaultAppPasswordOS, "false", "")
	})
	// Stop the container cleanly
	stopContainer(t, cli, id)
}

func TestDevConfigDisabled(t *testing.T) {
	t.Parallel()

	cli := ce.NewContainerClient()
	containerConfig := ce.ContainerConfig{
		Env: []string{
			"LICENSE=accept",
			"MQ_QMGR_NAME=qm1",
			"MQ_DEV=false",
		},
	}
	id := runContainerWithPorts(t, cli, &containerConfig, []int{9443})
	defer cleanContainer(t, cli, id)
	waitForReady(t, cli, id)
	waitForWebReady(t, cli, id, insecureTLSConfig)
	rc, _ := execContainer(t, cli, id, "", []string{"bash", "-c", "echo 'display qlocal(DEV*)' | runmqsc"})
	if rc == 0 {
		t.Errorf("Expected DEV queues to be missing")
	}
	// Stop the container cleanly
	stopContainer(t, cli, id)
}

// Test if SSLKEYR and CERTLABL attributes are not set when key and certificate
// are not supplied.
func TestSSLKEYRBlank(t *testing.T) {
	t.Parallel()

	cli := ce.NewContainerClient()
	containerConfig := ce.ContainerConfig{
		Env: []string{
			"LICENSE=accept",
			"MQ_QMGR_NAME=QM1",
			"MQ_ENABLE_EMBEDDED_WEB_SERVER=false",
		},
	}
	id := runContainerWithPorts(t, cli, &containerConfig, []int{9443})
	defer cleanContainer(t, cli, id)
	waitForReady(t, cli, id)

	// execute runmqsc to display qmgr SSLKEYR and CERTLABL attibutes.
	// Search the console output for exepcted values
	_, sslkeyROutput := execContainer(t, cli, id, "", []string{"bash", "-c", "echo 'DISPLAY QMGR SSLKEYR CERTLABL' | runmqsc"})
	if !strings.Contains(sslkeyROutput, "SSLKEYR( )") || !strings.Contains(sslkeyROutput, "CERTLABL( )") {
		// Although queue manager is ready, it may be that MQSC scripts have not been applied yet.
		// Hence wait for a second and retry few times before giving up.
		waitCount := 30
		var i int
		for i = 0; i < waitCount; i++ {
			time.Sleep(1 * time.Second)
			_, sslkeyROutput = execContainer(t, cli, id, "", []string{"bash", "-c", "echo 'DISPLAY QMGR SSLKEYR CERTLABL' | runmqsc"})
			if strings.Contains(sslkeyROutput, "SSLKEYR( )") && strings.Contains(sslkeyROutput, "CERTLABL( )") {
				break
			}
		}
		// Failed to get expected output? dump the contents of mqsc files.
		if i == waitCount {
			_, tls15mqsc := execContainer(t, cli, id, "", []string{"cat", "/etc/mqm/15-tls.mqsc"})
			_, autoMQSC := execContainer(t, cli, id, "", []string{"cat", "/mnt/mqm/data/qmgrs/QM1/autocfg/cached.mqsc"})
			t.Errorf("Expected SSLKEYR to be blank but it is not; got \"%v\"\n AutoConfig MQSC file contents %v\n 15-tls: %v", sslkeyROutput, autoMQSC, tls15mqsc)
		}
	}

	// Stop the container cleanly
	stopContainer(t, cli, id)
}

// Test if SSLKEYR and CERTLABL attributes are set when key and certificate
// are supplied.
func TestSSLKEYRWithSuppliedKeyAndCert(t *testing.T) {
	t.Parallel()

	cli := ce.NewContainerClient()

	containerConfig := ce.ContainerConfig{
		Env: []string{
			"LICENSE=accept",
			"MQ_QMGR_NAME=QM1",
			"MQ_ENABLE_EMBEDDED_WEB_SERVER=false",
		},
		Image: imageName(),
	}
	hostConfig := ce.ContainerHostConfig{
		Binds: []string{
			coverageBind(t),
			tlsDir(t, false) + ":/etc/mqm/pki/keys/default",
		},
	}
	networkingConfig := ce.ContainerNetworkSettings{}
	ID, err := cli.ContainerCreate(&containerConfig, &hostConfig, &networkingConfig, t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanContainer(t, cli, ID)
	startContainer(t, cli, ID)
	waitForReady(t, cli, ID)

	// execute runmqsc to display qmgr SSLKEYR and CERTLABL attibutes.
	// Search the console output for exepcted values
	_, sslkeyROutput := execContainer(t, cli, ID, "", []string{"bash", "-c", "echo 'DISPLAY QMGR SSLKEYR CERTLABL' | runmqsc"})
	if !strings.Contains(sslkeyROutput, "SSLKEYR(/run/runmqserver/tls/key)") || !strings.Contains(sslkeyROutput, "CERTLABL(default)") {
		// Although queue manager is ready, it may be that MQSC scripts have not been applied yet.
		// Hence wait for a second and retry few times before giving up.
		waitCount := 30
		var i int
		for i = 0; i < waitCount; i++ {
			time.Sleep(1 * time.Second)
			_, sslkeyROutput = execContainer(t, cli, ID, "", []string{"bash", "-c", "echo 'DISPLAY QMGR SSLKEYR CERTLABL' | runmqsc"})
			if strings.Contains(sslkeyROutput, "SSLKEYR(/run/runmqserver/tls/key)") && strings.Contains(sslkeyROutput, "CERTLABL(default)") {
				break
			}
		}
		// Failed to get expected output? dump the contents of mqsc files.
		if i == waitCount {
			_, tls15mqsc := execContainer(t, cli, ID, "", []string{"cat", "/etc/mqm/15-tls.mqsc"})
			_, autoMQSC := execContainer(t, cli, ID, "", []string{"cat", "/mnt/mqm/data/qmgrs/QM1/autocfg/cached.mqsc"})
			t.Errorf("Expected SSLKEYR to be '/run/runmqserver/tls/key' but it is not; got \"%v\" \n AutoConfig MQSC file contents %v\n 15-tls: %v", sslkeyROutput, autoMQSC, tls15mqsc)
		}
	}

	// Stop the container cleanly
	stopContainer(t, cli, ID)
}

// Test with CA cert
func TestSSLKEYRWithCACert(t *testing.T) {
	t.Parallel()

	cli := ce.NewContainerClient()

	containerConfig := ce.ContainerConfig{
		Env: []string{
			"LICENSE=accept",
			"MQ_QMGR_NAME=QM1",
			"MQ_ENABLE_EMBEDDED_WEB_SERVER=false",
		},
		Image: imageName(),
	}
	hostConfig := ce.ContainerHostConfig{
		Binds: []string{
			coverageBind(t),
			tlsDirWithCA(t, false) + ":/etc/mqm/pki/keys/QM1CA",
		},
	}
	// Assign a random port for the web server on the host
	var binding ce.PortBinding
	ports := []int{9443}
	for _, p := range ports {
		port := fmt.Sprintf("%v/tcp", p)
		binding = ce.PortBinding{
			ContainerPort: port,
			HostIP:        "0.0.0.0",
		}
		hostConfig.PortBindings = append(hostConfig.PortBindings, binding)
	}
	networkingConfig := ce.ContainerNetworkSettings{}
	ID, err := cli.ContainerCreate(&containerConfig, &hostConfig, &networkingConfig, t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanContainer(t, cli, ID)
	startContainer(t, cli, ID)
	waitForReady(t, cli, ID)

	// execute runmqsc to display qmgr SSLKEYR and CERTLABL attibutes.
	// Search the console output for exepcted values
	_, sslkeyROutput := execContainer(t, cli, ID, "", []string{"bash", "-c", "echo 'DISPLAY QMGR SSLKEYR CERTLABL' | runmqsc"})

	if !strings.Contains(sslkeyROutput, "SSLKEYR(/run/runmqserver/tls/key)") {
		// Although queue manager is ready, it may be that MQSC scripts have not been applied yet.
		// Hence wait for a second and retry few times before giving up.
		waitCount := 30
		var i int
		for i = 0; i < waitCount; i++ {
			time.Sleep(1 * time.Second)
			_, sslkeyROutput = execContainer(t, cli, ID, "", []string{"bash", "-c", "echo 'DISPLAY QMGR SSLKEYR CERTLABL' | runmqsc"})
			if strings.Contains(sslkeyROutput, "SSLKEYR(/run/runmqserver/tls/key)") {
				break
			}
		}
		// Failed to get expected output? dump the contents of mqsc files.
		if i == waitCount {
			_, tls15mqsc := execContainer(t, cli, ID, "", []string{"cat", "/etc/mqm/15-tls.mqsc"})
			_, autoMQSC := execContainer(t, cli, ID, "", []string{"cat", "/mnt/mqm/data/qmgrs/QM1/autocfg/cached.mqsc"})
			t.Errorf("Expected SSLKEYR to be '/run/runmqserver/tls/key' but it is not; got \"%v\"\n AutoConfig MQSC file contents %v\n 15-tls: %v", sslkeyROutput, autoMQSC, tls15mqsc)
		}
	}

	if !strings.Contains(sslkeyROutput, "CERTLABL(QM1CA)") {
		_, autoMQSC := execContainer(t, cli, ID, "", []string{"cat", "/etc/mqm/15-tls.mqsc"})
		t.Errorf("Expected CERTLABL to be 'QM1CA' but it is not; got \"%v\" \n MQSC File contents %v", sslkeyROutput, autoMQSC)
	}

	// Stop the container cleanly
	stopContainer(t, cli, ID)
}

// Verifies SSLFIPS is set to NO if MQ_ENABLE_FIPS=false
func TestSSLFIPSNO(t *testing.T) {
	t.Parallel()

	cli := ce.NewContainerClient()

	containerConfig := ce.ContainerConfig{
		Env: []string{
			"LICENSE=accept",
			"MQ_QMGR_NAME=QM1",
			"MQ_ENABLE_EMBEDDED_WEB_SERVER=false",
			"MQ_ENABLE_FIPS=false",
		},
		Image: imageName(),
	}
	hostConfig := ce.ContainerHostConfig{
		Binds: []string{
			coverageBind(t),
			tlsDir(t, false) + ":/etc/mqm/pki/keys/default",
		},
	}
	networkingConfig := ce.ContainerNetworkSettings{}
	ID, err := cli.ContainerCreate(&containerConfig, &hostConfig, &networkingConfig, t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanContainer(t, cli, ID)
	startContainer(t, cli, ID)
	waitForReady(t, cli, ID)

	// execute runmqsc to display qmgr SSLKEYR, SSLFIPS and CERTLABL attibutes.
	// Search the console output for exepcted values
	_, sslFIPSOutput := execContainer(t, cli, ID, "", []string{"bash", "-c", "echo 'DISPLAY QMGR SSLKEYR CERTLABL SSLFIPS' | runmqsc"})
	if !strings.Contains(sslFIPSOutput, "SSLKEYR(/run/runmqserver/tls/key)") {
		t.Errorf("Expected SSLKEYR to be '/run/runmqserver/tls/key' but it is not; got \"%v\"", sslFIPSOutput)
	}
	if !strings.Contains(sslFIPSOutput, "CERTLABL(default)") {
		t.Errorf("Expected CERTLABL to be 'default' but it is not; got \"%v\"", sslFIPSOutput)
	}

	if !strings.Contains(sslFIPSOutput, "SSLFIPS(NO)") {
		t.Errorf("Expected SSLFIPS to be 'NO' but it is not; got \"%v\"", sslFIPSOutput)
	}

	// Stop the container cleanly
	stopContainer(t, cli, ID)
}

// Verifies SSLFIPS is set to YES if certificates for queue manager
// are supplied and MQ_ENABLE_FIPS=true
func TestSSLFIPSYES(t *testing.T) {
	t.Parallel()

	cli := ce.NewContainerClient()

	appPassword := "differentPassw0rd"
	containerConfig := ce.ContainerConfig{
		Env: []string{
			"LICENSE=accept",
			"MQ_APP_PASSWORD=" + appPassword,
			"MQ_QMGR_NAME=QM1",
			"MQ_ENABLE_EMBEDDED_WEB_SERVER=false",
			"MQ_ENABLE_FIPS=true",
		},
		Image: imageName(),
	}
	hostConfig := ce.ContainerHostConfig{
		Binds: []string{
			coverageBind(t),
			tlsDir(t, false) + ":/etc/mqm/pki/keys/default",
		},
	}
	var binding ce.PortBinding
	ports := []int{1414}
	for _, p := range ports {
		port := fmt.Sprintf("%v/tcp", p)
		binding = ce.PortBinding{
			ContainerPort: port,
			HostIP:        "0.0.0.0",
		}
		hostConfig.PortBindings = append(hostConfig.PortBindings, binding)
	}
	networkingConfig := ce.ContainerNetworkSettings{}
	ID, err := cli.ContainerCreate(&containerConfig, &hostConfig, &networkingConfig, t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanContainer(t, cli, ID)
	startContainer(t, cli, ID)
	waitForReady(t, cli, ID)

	// Check for expected message on container log
	logs := inspectLogs(t, cli, ID)
	if !strings.Contains(logs, "FIPS cryptography is enabled.") {
		t.Errorf("Expected 'FIPS cryptography is enabled.' but got %v\n", logs)
	}

	// execute runmqsc to display qmgr SSLKEYR, SSLFIPS and CERTLABL attibutes.
	// Search the console output for exepcted values
	_, sslFIPSOutput := execContainer(t, cli, ID, "", []string{"bash", "-c", "echo 'DISPLAY QMGR SSLKEYR CERTLABL SSLFIPS' | runmqsc"})
	if !strings.Contains(sslFIPSOutput, "SSLKEYR(/run/runmqserver/tls/key)") {
		t.Errorf("Expected SSLKEYR to be '/run/runmqserver/tls/key' but it is not; got \"%v\"", sslFIPSOutput)
	}
	if !strings.Contains(sslFIPSOutput, "CERTLABL(default)") {
		t.Errorf("Expected CERTLABL to be 'default' but it is not; got \"%v\"", sslFIPSOutput)
	}

	if !strings.Contains(sslFIPSOutput, "SSLFIPS(YES)") {
		t.Errorf("Expected SSLFIPS to be 'YES' but it is not; got \"%v\"", sslFIPSOutput)
	}

	t.Run("JMS", func(t *testing.T) {
		// Run the JMS tests, with no password specified
		runJMSTests(t, cli, ID, true, "app", appPassword, "false", "TLS_RSA_WITH_AES_256_CBC_SHA256")
	})

	// Stop the container cleanly
	stopContainer(t, cli, ID)
}

// TestDevSecureFIPSYESWeb verifies if the MQ Web Server is running in FIPS mode
func TestDevSecureFIPSTrueWeb(t *testing.T) {
	t.Parallel()

	cli := ce.NewContainerClient()

	const tlsPassPhrase string = "passw0rd"
	qm := "qm1"
	appPassword := "differentPassw0rd"
	containerConfig := ce.ContainerConfig{
		Env: []string{
			"LICENSE=accept",
			"MQ_QMGR_NAME=" + qm,
			"MQ_APP_PASSWORD=" + appPassword,
			"DEBUG=1",
			"WLP_LOGGING_MESSAGE_FORMAT=JSON",
			"MQ_ENABLE_EMBEDDED_WEB_SERVER_LOG=true",
			"MQ_ENABLE_FIPS=true",
		},
		Image: imageName(),
	}
	hostConfig := ce.ContainerHostConfig{
		Binds: []string{
			coverageBind(t),
			tlsDir(t, false) + ":/etc/mqm/pki/keys/default",
			tlsDir(t, false) + ":/etc/mqm/pki/trust/default",
		},
	}
	// Assign a random port for the web server on the host
	// TODO: Don't do this for all tests
	var binding ce.PortBinding
	ports := []int{9443}
	for _, p := range ports {
		port := fmt.Sprintf("%v/tcp", p)
		binding = ce.PortBinding{
			ContainerPort: port,
			HostIP:        "0.0.0.0",
		}
		hostConfig.PortBindings = append(hostConfig.PortBindings, binding)
	}
	networkingConfig := ce.ContainerNetworkSettings{}
	ID, err := cli.ContainerCreate(&containerConfig, &hostConfig, &networkingConfig, t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanContainer(t, cli, ID)

	startContainer(t, cli, ID)
	waitForReady(t, cli, ID)
	cert := filepath.Join(tlsDir(t, true), "server.crt")
	waitForWebReady(t, cli, ID, createTLSConfig(t, cert, tlsPassPhrase))

	// Create a TLS Config with a cipher to use when connecting over HTTPS
	var secureTLSConfig *tls.Config = createTLSConfigWithCipher(t, cert, tlsPassPhrase, []uint16{tls.TLS_RSA_WITH_AES_256_GCM_SHA384})
	// Put a message to queue
	t.Run("REST messaging", func(t *testing.T) {
		testRESTMessaging(t, cli, ID, secureTLSConfig, qm, "app", appPassword, "")
	})

	// Create a TLS Config with a non-FIPS cipher to use when connecting over HTTPS
	var secureNonFIPSCipherConfig *tls.Config = createTLSConfigWithCipher(t, cert, tlsPassPhrase, []uint16{tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA})
	// Put a message to queue - the attempt to put message will fail with a EOF return message.
	t.Run("REST messaging", func(t *testing.T) {
		testRESTMessaging(t, cli, ID, secureNonFIPSCipherConfig, qm, "app", appPassword, "EOF")
	})

	// Stop the container cleanly
	stopContainer(t, cli, ID)
}

// TestDevSecureNOFIPSWeb verifies if the MQ Web Server is not running in FIPS mode
func TestDevSecureFalseFIPSWeb(t *testing.T) {
	t.Parallel()

	cli := ce.NewContainerClient()

	const tlsPassPhrase string = "passw0rd"
	qm := "qm1"
	appPassword := "differentPassw0rd"
	containerConfig := ce.ContainerConfig{
		Env: []string{
			"LICENSE=accept",
			"MQ_QMGR_NAME=" + qm,
			"MQ_APP_PASSWORD=" + appPassword,
			"DEBUG=1",
			"WLP_LOGGING_MESSAGE_FORMAT=JSON",
			"MQ_ENABLE_EMBEDDED_WEB_SERVER_LOG=true",
			"MQ_ENABLE_FIPS=false",
		},
		Image: imageName(),
	}
	hostConfig := ce.ContainerHostConfig{
		Binds: []string{
			coverageBind(t),
			tlsDir(t, false) + ":/etc/mqm/pki/keys/default",
			tlsDir(t, false) + ":/etc/mqm/pki/trust/default",
		},
	}
	// Assign a random port for the web server on the host
	var binding ce.PortBinding
	ports := []int{9443}
	for _, p := range ports {
		port := fmt.Sprintf("%v/tcp", p)
		binding = ce.PortBinding{
			ContainerPort: port,
			HostIP:        "0.0.0.0",
		}
		hostConfig.PortBindings = append(hostConfig.PortBindings, binding)
	}
	networkingConfig := ce.ContainerNetworkSettings{}
	ID, err := cli.ContainerCreate(&containerConfig, &hostConfig, &networkingConfig, t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanContainer(t, cli, ID)
	startContainer(t, cli, ID)
	waitForReady(t, cli, ID)

	cert := filepath.Join(tlsDir(t, true), "server.crt")
	waitForWebReady(t, cli, ID, createTLSConfig(t, cert, tlsPassPhrase))

	// As FIPS is not enabled, the MQ WebServer (actually Java) will choose a JSSE provider from the list
	// specified in java.security file. We will need to enable java.net.debug and then parse the web server
	// logs to check what JJSE provider is being used. Hence just check the jvm.options file does not contain
	// -Dcom.ibm.jsse2.usefipsprovider line.
	_, jvmOptionsOutput := execContainer(t, cli, ID, "", []string{"bash", "-c", "cat /var/mqm/web/installations/Installation1/servers/mqweb/configDropins/defaults/jvm.options"})
	if strings.Contains(jvmOptionsOutput, "-Dcom.ibm.jsse2.usefipsprovider") {
		t.Errorf("Did not expect -Dcom.ibm.jsse2.usefipsprovider but it is not; got \"%v\"", jvmOptionsOutput)
	}

	// Just do a HTTPS GET as well to query installation details.
	var secureTLSConfig *tls.Config = createTLSConfigWithCipher(t, cert, tlsPassPhrase, []uint16{tls.TLS_RSA_WITH_AES_256_GCM_SHA384})
	t.Run("REST admin", func(t *testing.T) {
		testRESTAdmin(t, cli, ID, secureTLSConfig, "")
	})

	// Stop the container cleanly
	stopContainer(t, cli, ID)
}

// Verify SSLFIPS is set to NO if no certificates were supplied
func TestSSLFIPSTrueNoCerts(t *testing.T) {
	t.Parallel()

	cli := ce.NewContainerClient()

	appPassword := "differentPassw0rd"
	containerConfig := ce.ContainerConfig{
		Env: []string{
			"LICENSE=accept",
			"MQ_APP_PASSWORD=" + appPassword,
			"MQ_QMGR_NAME=QM1",
			"MQ_ENABLE_EMBEDDED_WEB_SERVER=false",
			"MQ_ENABLE_FIPS=true",
		},
		Image: imageName(),
	}
	hostConfig := ce.ContainerHostConfig{
		Binds: []string{
			coverageBind(t),
		},
	}
	networkingConfig := ce.ContainerNetworkSettings{}
	ID, err := cli.ContainerCreate(&containerConfig, &hostConfig, &networkingConfig, t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanContainer(t, cli, ID)
	startContainer(t, cli, ID)
	waitForReady(t, cli, ID)

	// execute runmqsc to display qmgr SSLKEYR, SSLFIPS and CERTLABL attibutes.
	// Search the console output for exepcted values
	_, sslFIPSOutput := execContainer(t, cli, ID, "", []string{"bash", "-c", "echo 'DISPLAY QMGR SSLKEYR CERTLABL SSLFIPS' | runmqsc"})
	if !strings.Contains(sslFIPSOutput, "SSLKEYR( )") {
		t.Errorf("Expected SSLKEYR to be ' ' but it is not; got \"%v\"", sslFIPSOutput)
	}
	if !strings.Contains(sslFIPSOutput, "CERTLABL( )") {
		t.Errorf("Expected CERTLABL to be blank but it is not; got \"%v\"", sslFIPSOutput)
	}

	if !strings.Contains(sslFIPSOutput, "SSLFIPS(NO)") {
		t.Errorf("Expected SSLFIPS to be 'NO' but it is not; got \"%v\"", sslFIPSOutput)
	}

	// Stop the container cleanly
	stopContainer(t, cli, ID)
}

// Verifies SSLFIPS is set to NO if MQ_ENABLE_FIPS=tru (invalid value)
func TestSSLFIPSInvalidValue(t *testing.T) {
	t.Parallel()

	cli := ce.NewContainerClient()

	containerConfig := ce.ContainerConfig{
		Env: []string{
			"LICENSE=accept",
			"MQ_QMGR_NAME=QM1",
			"MQ_ENABLE_EMBEDDED_WEB_SERVER=false",
			"MQ_ENABLE_FIPS=tru",
		},
		Image: imageName(),
	}
	hostConfig := ce.ContainerHostConfig{
		Binds: []string{
			coverageBind(t),
			tlsDir(t, false) + ":/etc/mqm/pki/keys/default",
		},
	}
	networkingConfig := ce.ContainerNetworkSettings{}
	ID, err := cli.ContainerCreate(&containerConfig, &hostConfig, &networkingConfig, t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanContainer(t, cli, ID)
	startContainer(t, cli, ID)
	waitForReady(t, cli, ID)

	// execute runmqsc to display qmgr SSLKEYR, SSLFIPS and CERTLABL attibutes.
	// Search the console output for exepcted values
	_, sslFIPSOutput := execContainer(t, cli, ID, "", []string{"bash", "-c", "echo 'DISPLAY QMGR SSLKEYR CERTLABL SSLFIPS' | runmqsc"})
	if !strings.Contains(sslFIPSOutput, "SSLKEYR(/run/runmqserver/tls/key)") {
		t.Errorf("Expected SSLKEYR to be '/run/runmqserver/tls/key' but it is not; got \"%v\"", sslFIPSOutput)
	}

	if !strings.Contains(sslFIPSOutput, "CERTLABL(default)") {
		t.Errorf("Expected CERTLABL to be 'default' but it is not; got \"%v\"", sslFIPSOutput)
	}

	if !strings.Contains(sslFIPSOutput, "SSLFIPS(NO)") {
		t.Errorf("Expected SSLFIPS to be 'NO' but it is not; got \"%v\"", sslFIPSOutput)
	}

	// Stop the container cleanly
	stopContainer(t, cli, ID)
}

// Container creation fails when invalid certs are passed and MQ_ENABLE_FIPS set true
func TestSSLFIPSBadCerts(t *testing.T) {
	t.Parallel()

	cli := ce.NewContainerClient()

	containerConfig := ce.ContainerConfig{
		Env: []string{
			"LICENSE=accept",
			"MQ_QMGR_NAME=QM1",
			"MQ_ENABLE_EMBEDDED_WEB_SERVER=false",
			"MQ_ENABLE_FIPS=true",
		},
		Image: imageName(),
	}
	hostConfig := ce.ContainerHostConfig{
		Binds: []string{
			coverageBind(t),
			tlsDirInvalid(t, false) + ":/etc/mqm/pki/keys/default",
		},
	}
	networkingConfig := ce.ContainerNetworkSettings{}
	ID, err := cli.ContainerCreate(&containerConfig, &hostConfig, &networkingConfig, t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanContainer(t, cli, ID)
	startContainer(t, cli, ID)

	rc := waitForContainer(t, cli, ID, 20*time.Second)
	// Expect return code 1 if container failed to create.
	if rc == 1 {
		// Get container logs and search for specific message.
		logs := inspectLogs(t, cli, ID)
		if strings.Contains(logs, "Failed to parse private key") {
			t.Logf("Container creating failed because of invalid certifates")
		}
	} else {
		// Some other error occurred.
		t.Errorf("Expected rc=0, got rc=%v", rc)
	}

	// Stop the container cleanly
	stopContainer(t, cli, ID)
}
