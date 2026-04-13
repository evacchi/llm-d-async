package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	k8slog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/redis/go-redis/v9"
	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/util/env"
	testutils "sigs.k8s.io/gateway-api-inference-extension/test/utils"
)

const (
	kindClusterName = "e2e-integration-tests"
	nsName          = "e2e-integration"

	// Manifests
	redisManifest          = "./yaml/redis.yaml"
	simManifest            = "./yaml/sim.yaml"
	eppManifest            = "./yaml/epp.yaml"
	prometheusManifest     = "./yaml/prometheus.yaml"
	asyncProcessorManifest = "./yaml/async-processor.yaml"
)

var (
	redisPort   string = env.GetEnvString("E2E_INTEGRATION_REDIS_PORT", "30480", ginkgo.GinkgoLogr)
	promPort    string = env.GetEnvString("E2E_INTEGRATION_PROM_PORT", "30491", ginkgo.GinkgoLogr)
	simPort     string = env.GetEnvString("E2E_INTEGRATION_SIM_PORT", "30490", ginkgo.GinkgoLogr)
	envoyPort   string = env.GetEnvString("E2E_INTEGRATION_ENVOY_PORT", "30492", ginkgo.GinkgoLogr)

	containerRuntime = env.GetEnvString("CONTAINER_TOOL", env.GetEnvString("CONTAINER_RUNTIME", "docker", ginkgo.GinkgoLogr), ginkgo.GinkgoLogr)
	apImage          = env.GetEnvString("AP_IMAGE", "ghcr.io/llm-d-incubation/async-processor:e2e-test", ginkgo.GinkgoLogr)
	eppImage         = env.GetEnvString("EPP_IMAGE", "epp:e2e-integration", ginkgo.GinkgoLogr)
	simImage         = env.GetEnvString("SIM_IMAGE", "llm-d-inference-sim:e2e-integration", ginkgo.GinkgoLogr)
	gaieRoot         = env.GetEnvString("GAIE_ROOT", defaultGaieRoot(), ginkgo.GinkgoLogr)
	simRoot          = env.GetEnvString("SIM_ROOT", defaultSimRoot(), ginkgo.GinkgoLogr)

	testConfig *testutils.TestConfig

	rdb         *redis.Client
	promURL     string
	simAdminURL string
	envoyURL    string
)

func TestIntegration(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Integration Test Suite")
}

var _ = ginkgo.BeforeSuite(func() {
	setupK8sCluster()
	testConfig = testutils.NewTestConfig(nsName, "")
	setupK8sClient()
	setupNamespace()
	applyManifests()
	setupClients()
})

var _ = ginkgo.AfterSuite(func() {
	if rdb != nil {
		rdb.Close() //nolint:errcheck
	}

	skipCleanup := env.GetEnvString("E2E_SKIP_CLEANUP", "false", ginkgo.GinkgoLogr)
	if skipCleanup == "true" {
		fmt.Println("Skipping cluster cleanup (E2E_SKIP_CLEANUP=true)")
		return
	}

	ginkgo.By("Deleting kind cluster " + kindClusterName)
	command := exec.Command("kind", "delete", "cluster", "--name", kindClusterName)
	session, err := gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	if err != nil {
		ginkgo.GinkgoLogr.Error(err, "Failed to delete kind cluster")
	} else {
		gomega.Eventually(session).WithTimeout(60 * time.Second).Should(gexec.Exit())
	}
})

func setupK8sCluster() {
	ginkgo.By("Creating Kind cluster " + kindClusterName)
	command := exec.Command("kind", "create", "cluster", "--name", kindClusterName, "--wait", "120s", "--config", "-")
	stdin, err := command.StdinPipe()
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	go func() {
		defer func() {
			gomega.Expect(stdin.Close()).To(gomega.Succeed())
		}()
		cfg := strings.ReplaceAll(kindClusterConfig, "${REDIS_PORT}", redisPort)
		cfg = strings.ReplaceAll(cfg, "${PROM_PORT}", promPort)
		cfg = strings.ReplaceAll(cfg, "${SIM_PORT}", simPort)
		cfg = strings.ReplaceAll(cfg, "${ENVOY_PORT}", envoyPort)
		_, err := io.WriteString(stdin, cfg)
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	}()
	session, err := gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Eventually(session).WithTimeout(600 * time.Second).Should(gexec.Exit(0))

	ginkgo.By("Building async-processor image")
	command = exec.Command(containerRuntime, "build", "-t", apImage, projectRoot())
	session, err = gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Eventually(session).WithTimeout(600 * time.Second).Should(gexec.Exit(0))

	ginkgo.By("Building EPP image from " + gaieRoot)
	command = exec.Command(containerRuntime, "build", "-t", eppImage, gaieRoot)
	session, err = gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Eventually(session).WithTimeout(600 * time.Second).Should(gexec.Exit(0))

	ginkgo.By("Building sim image from " + simRoot)
	command = exec.Command(containerRuntime, "build", "-t", simImage, simRoot)
	session, err = gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Eventually(session).WithTimeout(600 * time.Second).Should(gexec.Exit(0))

	kindLoadImage(apImage)
	kindLoadImage(eppImage)
	kindLoadImage(simImage)

	ginkgo.By("Pulling redis:7-alpine")
	command = exec.Command(containerRuntime, "pull", "redis:7-alpine")
	session, err = gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Eventually(session).WithTimeout(300 * time.Second).Should(gexec.Exit(0))
	kindLoadImage("redis:7-alpine")

	ginkgo.By("Pulling docker.io/envoyproxy/envoy:distroless-v1.33.2")
	command = exec.Command(containerRuntime, "pull", "docker.io/envoyproxy/envoy:distroless-v1.33.2")
	session, err = gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Eventually(session).WithTimeout(300 * time.Second).Should(gexec.Exit(0))
	kindLoadImage("docker.io/envoyproxy/envoy:distroless-v1.33.2")

	ginkgo.By("Pulling prom/prometheus:v2.53.0")
	command = exec.Command(containerRuntime, "pull", "prom/prometheus:v2.53.0")
	session, err = gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Eventually(session).WithTimeout(300 * time.Second).Should(gexec.Exit(0))
	kindLoadImage("prom/prometheus:v2.53.0")

	// The sim image is pulled with imagePullPolicy: Always directly by the cluster.
}

func kindLoadImage(image string) {
	ginkgo.By(fmt.Sprintf("Loading %s into cluster %s", image, kindClusterName))
	command := exec.Command("kind", "load", "docker-image", image, "--name", kindClusterName)
	session, err := gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Eventually(session).WithTimeout(300 * time.Second).Should(gexec.Exit(0))
}

func setupK8sClient() {
	k8sCfg, err := config.GetConfigWithContext("")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.ExpectWithOffset(1, k8sCfg).NotTo(gomega.BeNil())

	err = clientgoscheme.AddToScheme(testConfig.Scheme)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	testConfig.CreateCli()
	k8slog.SetLogger(ginkgo.GinkgoLogr)
}

func setupNamespace() {
	_, err := testConfig.KubeCli.CoreV1().Namespaces().Get(testConfig.Context, nsName, metav1.GetOptions{})
	if err == nil {
		return
	}
	gomega.Expect(errors.IsNotFound(err)).To(gomega.BeTrue())

	ginkgo.By("Creating namespace " + nsName)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}
	_, err = testConfig.KubeCli.CoreV1().Namespaces().Create(testConfig.Context, ns, metav1.CreateOptions{})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

func applyManifests() {
	// All manifests are applied via kubectl to avoid scheme registration issues
	// with GAIE custom resources (InferencePool, CRDs).
	ginkgo.By("Applying InferencePool CRDs")
	for _, crd := range inferencePoolCRDs() {
		kubectlApplyFile(crd, nil)
	}

	ginkgo.By("Applying Redis manifest")
	kubectlApplyFile(redisManifest, nil)

	ginkgo.By("Applying sim manifest")
	kubectlApplyFile(simManifest, map[string]string{"${SIM_IMAGE}": simImage})

	ginkgo.By("Applying EPP manifest")
	kubectlApplyFile(eppManifest, map[string]string{"${EPP_IMAGE}": eppImage})

	ginkgo.By("Applying Prometheus manifest")
	kubectlApplyFile(prometheusManifest, nil)

	ginkgo.By("Applying Envoy manifest")
	envoyManifest := filepath.Join(gaieRoot, "test", "testdata", "envoy.yaml")
	kubectlApplyFileInNamespace(envoyManifest, nsName, map[string]string{
		"$E2E_NS":            nsName,
		"vllm-qwen3-32b-epp": "epp-svc",
	})
	// Patch Envoy service to NodePort so test code can reach it for probe requests.
	kubectlPatchEnvoyNodePort()

	ginkgo.By("Applying async-processor manifest")
	kubectlApplyFile(asyncProcessorManifest, map[string]string{"${AP_IMAGE}": apImage})
}

// kubectlPatchEnvoyNodePort patches the Envoy service to NodePort so test code
// outside the cluster can reach it for probe requests.
func kubectlPatchEnvoyNodePort() {
	patch := fmt.Sprintf(`{"spec":{"type":"NodePort","ports":[{"name":"http-8081","port":8081,"targetPort":8081,"nodePort":%s}]}}`, envoyPort)
	command := exec.Command("kubectl", "--context", "kind-"+kindClusterName,
		"-n", nsName, "patch", "service", "envoy",
		"--type=merge", "--patch", patch)
	session, err := gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Eventually(session).WithTimeout(30 * time.Second).Should(gexec.Exit(0))
}

// kubectlApplyFile applies a YAML manifest via kubectl, optionally substituting
// template variables. Uses stdin to avoid temp files.
func kubectlApplyFile(path string, substitutions map[string]string) {
	kubectlApplyFileInNamespace(path, "", substitutions)
}

func kubectlApplyFileInNamespace(path, namespace string, substitutions map[string]string) {
	content, err := os.ReadFile(path)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred(), "reading manifest %s", path)

	yaml := string(content)
	for k, v := range substitutions {
		yaml = strings.ReplaceAll(yaml, k, v)
	}

	args := []string{"--context", "kind-" + kindClusterName, "apply"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	args = append(args, "-f", "-")

	command := exec.Command("kubectl", args...)
	command.Stdin = strings.NewReader(yaml)
	session, err := gexec.Start(command, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
	gomega.Eventually(session).WithTimeout(60 * time.Second).Should(gexec.Exit(0))
}

func setupClients() {
	promURL     = "http://localhost:" + promPort
	simAdminURL = "http://localhost:" + simPort
	envoyURL    = "http://localhost:" + envoyPort

	ginkgo.By("Creating Redis client on localhost:" + redisPort)
	rdb = redis.NewClient(&redis.Options{Addr: "localhost:" + redisPort})
	gomega.Eventually(func() error {
		return rdb.Ping(context.Background()).Err()
	}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())

	ginkgo.By("Waiting for Prometheus to be ready")
	gomega.Eventually(func() error {
		resp, err := http.Get(promURL + "/-/ready")
		if err != nil {
			return err
		}
		return resp.Body.Close()
	}, 60*time.Second, 2*time.Second).Should(gomega.Succeed())

	ginkgo.By("Waiting for sim to be ready")
	gomega.Eventually(func() error {
		resp, err := http.Get(simAdminURL + "/metrics")
		if err != nil {
			return err
		}
		return resp.Body.Close()
	}, 60*time.Second, 2*time.Second).Should(gomega.Succeed())

	ginkgo.By("Waiting for Envoy to be ready")
	gomega.Eventually(func() error {
		resp, err := http.Get(envoyURL + "/v1/completions")
		if err != nil {
			return err
		}
		resp.Body.Close() //nolint:errcheck
		return nil
	}, 60*time.Second, 2*time.Second).Should(gomega.Succeed())
}

// inferencePoolCRDs returns paths to the CRD files from the local GAIE checkout.
func inferencePoolCRDs() []string {
	base := filepath.Join(gaieRoot, "config", "crd", "bases")
	return []string{
		filepath.Join(base, "inference.networking.k8s.io_inferencepools.yaml"),
		filepath.Join(base, "inference.networking.x-k8s.io_inferenceobjectives.yaml"),
		filepath.Join(base, "inference.networking.x-k8s.io_inferencemodelrewrites.yaml"),
		filepath.Join(base, "inference.networking.x-k8s.io_inferencepoolimports.yaml"),
	}
}

func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "..")
}

func defaultGaieRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "..",
		"kubernetes-sigs", "gateway-api-inference-extension")
}

func defaultSimRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "..",
		"llm-d", "llm-d-inference-sim")
}

func substituteMany(inputs []string, substitutions map[string]string) []string {
	outputs := make([]string, len(inputs))
	for i, input := range inputs {
		output := input
		for key, value := range substitutions {
			output = strings.ReplaceAll(output, key, value)
		}
		outputs[i] = output
	}
	return outputs
}

const kindClusterConfig = `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- extraPortMappings:
  - containerPort: 30480
    hostPort: ${REDIS_PORT}
    protocol: TCP
  - containerPort: 30491
    hostPort: ${PROM_PORT}
    protocol: TCP
  - containerPort: 30490
    hostPort: ${SIM_PORT}
    protocol: TCP
  - containerPort: 30492
    hostPort: ${ENVOY_PORT}
    protocol: TCP
`
