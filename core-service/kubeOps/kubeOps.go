package kubeOps

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/jmoiron/sqlx"
	"github.com/lu1a/lcaas/core-service/db"
	"github.com/lu1a/lcaas/core-service/types"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	//
	// Uncomment to load all auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth"
	//
	// Or uncomment to load specific auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

func CreateNamespaceForNewProject(kubeClients []types.ContainerZone, project types.Project) error {
	for _, client := range kubeClients {
		clientset := client.ClientSet
		nsName := &apiv1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: project.NamespaceName(),
			},
		}
		_, err := clientset.CoreV1().Namespaces().Create(context.Background(), nsName, metav1.CreateOptions{})
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return err
		}

		defaultLimitRange := &apiv1.LimitRange{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default-limitrange",
			},
			Spec: apiv1.LimitRangeSpec{
				Limits: []apiv1.LimitRangeItem{{
					Type: apiv1.LimitTypeContainer,
					Default: apiv1.ResourceList{
						apiv1.ResourceCPU:    resource.MustParse("100m"),
						apiv1.ResourceMemory: resource.MustParse("256Mi"),
					},
					DefaultRequest: apiv1.ResourceList{
						apiv1.ResourceCPU:    resource.MustParse("100m"),
						apiv1.ResourceMemory: resource.MustParse("256Mi"),
					},
				}},
			},
		}
		_, err = clientset.CoreV1().LimitRanges(project.NamespaceName()).Create(context.Background(), defaultLimitRange, metav1.CreateOptions{})
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return err
		}
	}

	return nil
}

func CreateContainerFromClaim(log log.Logger, adminDB *sqlx.DB, kubeClients []types.ContainerZone, project types.Project, containerClaim types.ContainerClaim, areWeRecreating bool) error {
	err := db.SetContainerAsActivating(adminDB, containerClaim)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	err = createKubeResourcesForContainer(log, adminDB, kubeClients, project, containerClaim, areWeRecreating)
	if err != nil {
		log.Error(err.Error())
		dberr := db.SetContainerAsErrorState(adminDB, containerClaim)
		if dberr != nil {
			log.Error(dberr.Error())
			return dberr
		}
		return err
	}
	err = db.SetContainerAsActive(adminDB, containerClaim)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	return nil
}

func createKubeResourcesForContainer(log log.Logger, adminDB *sqlx.DB, kubeClients []types.ContainerZone, project types.Project, containerClaim types.ContainerClaim, areWeRecreating bool) error {
	namespace := project.NamespaceName()
	log.Debug("Creating namespace if not exists", "namespace", namespace)
	err := CreateNamespaceForNewProject(kubeClients, project)
	if err != nil {
		return err
	}

	addedResourcesToRollBack := []addedResourceToRollBack{}

	podEnvVarSpec := []apiv1.EnvVar{}

	for _, client := range kubeClients {
		if !slices.Contains(containerClaim.Zones, client.Name) {
			continue
		}
		log.Debug("Creating env vars")
		clientset := client.ClientSet
		deploymentsClient := clientset.AppsV1().Deployments(namespace)

		for _, envVar := range containerClaim.EnvVars {

			if !areWeRecreating {
				secret := &apiv1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: containerClaim.EnvVarSpecName(envVar.Name),
					},
					StringData: map[string]string{
						envVar.Name: envVar.Value,
					},
				}

				createdSecret, err := clientset.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
				if err != nil && !strings.Contains(err.Error(), "already exists") {
					rollbackErr := rollBackCreation(log, kubeClients, addedResourcesToRollBack)
					if rollbackErr != nil {
						return rollbackErr
					}
					return err
				}
				addedResourcesToRollBack = append(addedResourcesToRollBack, addedResourceToRollBack{
					zone:         client.Name,
					resourceType: "secret",
					namespace:    namespace,
					name:         containerClaim.EnvVarSpecName(envVar.Name),
				})

				log.Debug("Secret created", "secret", createdSecret.Name)
			} else {
				log.Debug("Secret not created because we're recreating this container", "container", containerClaim.Name, "secret", envVar.Name)
			}

			podEnvVarSpec = append(podEnvVarSpec, apiv1.EnvVar{
				Name: envVar.Name,
				ValueFrom: &apiv1.EnvVarSource{
					SecretKeyRef: &apiv1.SecretKeySelector{
						Key: envVar.Name,
						LocalObjectReference: apiv1.LocalObjectReference{
							Name: containerClaim.EnvVarSpecName(envVar.Name),
						},
					},
				},
			})
		}

		createdImagePullSecret := false
		// create image pull secret so that we can actually pull image from private repo
		if containerClaim.ImagePullSecret != nil && containerClaim.ImagePullSecret.URL != "" && !areWeRecreating {
			ipsName := containerClaim.EnvVarSpecName("image-pull-secret")

			var encodedFullCreds []byte
			if (containerClaim.ImagePullSecret.Username == "" || containerClaim.ImagePullSecret.Password == "") && containerClaim.ImagePullSecret.Token != "" { // just gonna only be able to use the token
				encodedFullCreds = []byte(fmt.Sprintf("{\"auths\":{\"%s\":{\"auth\":\"%s\"}}}", containerClaim.ImagePullSecret.URL, containerClaim.ImagePullSecret.Token))

			} else if containerClaim.ImagePullSecret.Token == "" { // or, if username and password both exist but there's no token
				encodedUserAndPass := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", containerClaim.ImagePullSecret.Username, containerClaim.ImagePullSecret.Password)))
				encodedFullCreds = []byte(fmt.Sprintf("{\"auths\":{\"%s\":{\"username\":\"%s\",\"password\":\"%s\",\"email\":\"%s\",\"auth\":\"%s\"}}}", containerClaim.ImagePullSecret.URL, containerClaim.ImagePullSecret.Username, containerClaim.ImagePullSecret.Password, containerClaim.ImagePullSecret.Email, encodedUserAndPass))

			} else if containerClaim.ImagePullSecret.Token != "" { // or, if username and password both exist AND there's also a token
				encodedFullCreds = []byte(fmt.Sprintf("{\"auths\":{\"%s\":{\"username\":\"%s\",\"password\":\"%s\",\"email\":\"%s\",\"auth\":\"%s\"}}}", containerClaim.ImagePullSecret.URL, containerClaim.ImagePullSecret.Username, containerClaim.ImagePullSecret.Password, containerClaim.ImagePullSecret.Email, containerClaim.ImagePullSecret.Token))

			} else { // error out if there's some random case
				rollbackErr := rollBackCreation(log, kubeClients, addedResourcesToRollBack)
				if rollbackErr != nil {
					return rollbackErr
				}
				return fmt.Errorf("Something went wrong when creating ImagePullSecret")
			}

			imagePullSecret := &apiv1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: ipsName,
				},
				Type: apiv1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					".dockerconfigjson": []byte(encodedFullCreds),
				},
			}

			_, err := clientset.CoreV1().Secrets(namespace).Create(context.Background(), imagePullSecret, metav1.CreateOptions{})
			if err != nil && !strings.Contains(err.Error(), "already exists") {
				rollbackErr := rollBackCreation(log, kubeClients, addedResourcesToRollBack)
				if rollbackErr != nil {
					return rollbackErr
				}
				return err
			}
			addedResourcesToRollBack = append(addedResourcesToRollBack, addedResourceToRollBack{
				zone:         client.Name,
				resourceType: "secret",
				namespace:    namespace,
				name:         ipsName,
			})
		} else if containerClaim.ImagePullSecret != nil && areWeRecreating { // don't create any secrets if we're recreating, just use what already exists since we don't know the secret values anymore
			createdImagePullSecret = true
		}

		containerSelectorName := ""
		var containerPorts []apiv1.ContainerPort
		for _, targetPort := range containerClaim.TargetPorts {
			containerPorts = append(containerPorts, apiv1.ContainerPort{
				Name:          "http",
				Protocol:      apiv1.ProtocolTCP,
				ContainerPort: int32(targetPort),
			})
		}

		if containerClaim.RunType == "once" {
			containerSelectorName = containerClaim.JobName()
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: containerSelectorName,
				},
				Spec: batchv1.JobSpec{
					Template: apiv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name: containerClaim.JobName(),
							Labels: map[string]string{
								"name": containerSelectorName,
							},
						},
						Spec: apiv1.PodSpec{
							Containers: []apiv1.Container{
								{
									Name:  containerSelectorName,
									Image: containerClaim.WholeImageWithTag(),
									Env:   podEnvVarSpec,
									Ports: containerPorts,

									Resources: apiv1.ResourceRequirements{
										Requests: apiv1.ResourceList{
											apiv1.ResourceCPU:    resource.MustParse(containerClaim.CPUMilliCoresAsResourceListStr()),
											apiv1.ResourceMemory: resource.MustParse(containerClaim.MemoryMBAsResourceListStr()),
										},
										Limits: apiv1.ResourceList{
											apiv1.ResourceCPU:    resource.MustParse(containerClaim.CPUMilliCoresAsResourceListStr()),
											apiv1.ResourceMemory: resource.MustParse(containerClaim.MemoryMBAsResourceListStr()),
										},
									},
								},
							},
							RestartPolicy: "Never",
						},
					},
				},
			}
			if containerClaim.Command != nil {
				job.Spec.Template.Spec.Containers[0].Command = containerClaim.Command
			}
			if createdImagePullSecret {
				job.Spec.Template.Spec.ImagePullSecrets = []apiv1.LocalObjectReference{{
					Name: containerClaim.EnvVarSpecName("image-pull-secret"),
				}}
			}

			log.Debug("Creating job if not exists", "job", containerClaim.Name, "zone", client.Name)
			_, err := clientset.BatchV1().Jobs(namespace).Create(context.Background(), job, metav1.CreateOptions{})
			if err != nil {
				return err
			}
		} else {
			containerSelectorName = containerClaim.DeploymentName()
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: containerSelectorName,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: int32Ptr(1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": containerSelectorName,
						},
					},
					Template: apiv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name: containerSelectorName,
							Labels: map[string]string{
								"app":  containerSelectorName,
								"name": containerSelectorName,
							},
						},
						Spec: apiv1.PodSpec{
							Containers: []apiv1.Container{
								{
									Name:  containerSelectorName,
									Image: containerClaim.WholeImageWithTag(),
									Env:   podEnvVarSpec,
									Ports: containerPorts,

									Resources: apiv1.ResourceRequirements{
										Requests: apiv1.ResourceList{
											apiv1.ResourceCPU:    resource.MustParse(containerClaim.CPUMilliCoresAsResourceListStr()),
											apiv1.ResourceMemory: resource.MustParse(containerClaim.MemoryMBAsResourceListStr()),
										},
										Limits: apiv1.ResourceList{
											apiv1.ResourceCPU:    resource.MustParse(containerClaim.CPUMilliCoresAsResourceListStr()),
											apiv1.ResourceMemory: resource.MustParse(containerClaim.MemoryMBAsResourceListStr()),
										},
									},
								},
							},
						},
					},
				},
			}
			if containerClaim.Command != nil {
				deployment.Spec.Template.Spec.Containers[0].Command = containerClaim.Command
			}
			if createdImagePullSecret {
				deployment.Spec.Template.Spec.ImagePullSecrets = []apiv1.LocalObjectReference{{
					Name: containerClaim.EnvVarSpecName("image-pull-secret"),
				}}
			}

			log.Debug("Creating deployment if not exists", "deployment", containerClaim.Name, "zone", client.Name)
			_, err := deploymentsClient.Create(context.Background(), deployment, metav1.CreateOptions{})
			if err != nil {
				rollbackErr := rollBackCreation(log, kubeClients, addedResourcesToRollBack)
				if rollbackErr != nil {
					return rollbackErr
				}
				return err
			}

			addedResourcesToRollBack = append(addedResourcesToRollBack, addedResourceToRollBack{
				zone:         client.Name,
				resourceType: "deployment",
				namespace:    namespace,
				name:         containerSelectorName,
			})
		}
		// the IP of the node this pod is on, so that we can use that node as the service too
		hostIP := ""

		// wait until the HostIP is set before initialising the service
		maxPingAttempts := 30 // will turn into 30 seconds of wait time
		for range maxPingAttempts {
			// have to "list" instead of "get" because the client API sucks
			pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("name=%s", containerSelectorName)})
			if err != nil {
				hostIP = client.DefaultRoutingIP
				break
			}
			if len(pods.Items) == 0 {
				hostIP = client.DefaultRoutingIP
				break
			}
			actualPod := pods.Items[0]
			if actualPod.Status.Phase == apiv1.PodFailed {
				hostIP = client.DefaultRoutingIP
				break
			}
			if len(actualPod.Status.HostIP) > 0 {
				hostIP = actualPod.Status.HostIP
				break
			}
			time.Sleep(1 * time.Second)
		}
		// 👆 all that code just to find the damn IP of the node, geez

		err = db.SaveNodeIPOfRunningContainer(adminDB, project, containerClaim, hostIP)
		if err != nil {
			rollbackErr := rollBackCreation(log, kubeClients, addedResourcesToRollBack)
			if rollbackErr != nil {
				return rollbackErr
			}
			return err
		}

		for _, targetPort := range containerClaim.TargetPorts {
			containerPorts = append(containerPorts, apiv1.ContainerPort{
				Name:          "http",
				Protocol:      apiv1.ProtocolTCP,
				ContainerPort: int32(targetPort),
			})

			realLifePortForBigBoys, err := db.FindRandomFreePortAndSave(adminDB, project, containerClaim, targetPort)
			if err != nil {
				rollbackErr := rollBackCreation(log, kubeClients, addedResourcesToRollBack)
				if rollbackErr != nil {
					return rollbackErr
				}
				return err
			}

			service := apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      containerClaim.ServiceName(targetPort),
					Namespace: namespace,
					Labels: map[string]string{
						"app": containerClaim.ServiceName(targetPort),
					},
				},
				Spec: apiv1.ServiceSpec{
					Ports: []apiv1.ServicePort{
						{
							Protocol:   apiv1.ProtocolTCP,
							Port:       int32(realLifePortForBigBoys),
							TargetPort: intstr.FromInt32(int32(targetPort)),
						},
					},
					Selector: map[string]string{
						"app": containerSelectorName,
					},
					Type:        apiv1.ServiceTypeLoadBalancer,
					ExternalIPs: []string{hostIP},
				},
			}

			log.Debug("Creating service if not exists", "service", containerClaim.ServiceName(targetPort), "zone", client.Name)
			_, err = clientset.CoreV1().Services(namespace).Create(context.Background(), &service, metav1.CreateOptions{})
			if err != nil {
				rollbackErr := rollBackCreation(log, kubeClients, addedResourcesToRollBack)
				if rollbackErr != nil {
					return rollbackErr
				}
				return err
			}
			addedResourcesToRollBack = append(addedResourcesToRollBack, addedResourceToRollBack{
				zone:         client.Name,
				resourceType: "service",
				namespace:    namespace,
				name:         containerClaim.ServiceName(targetPort),
			})
		}

		// TODO: outside-of-cluster nginx proxy?
	}

	return nil
}

func rollBackCreation(log log.Logger, kubeClients []types.ContainerZone, addedResourcesToRollBack []addedResourceToRollBack) error {
	deletePolicy := metav1.DeletePropagationForeground

	log.Info("🧹 Rolling back a failing half-creation")

	for _, resourceToRollBack := range addedResourcesToRollBack {
		for _, client := range kubeClients {
			if resourceToRollBack.zone != client.Name {
				continue
			}

			clientset := client.ClientSet

			switch resourceToRollBack.resourceType {
			case "secret":
				if err := clientset.CoreV1().Secrets(resourceToRollBack.namespace).Delete(context.TODO(), resourceToRollBack.name, metav1.DeleteOptions{
					PropagationPolicy: &deletePolicy,
				}); err != nil {
					return err
				}
				log.Info("Deleted secret (env var)")

			case "deployment":
				deploymentsClient := clientset.AppsV1().Deployments(resourceToRollBack.namespace)
				if err := deploymentsClient.Delete(context.Background(), resourceToRollBack.name, metav1.DeleteOptions{
					PropagationPolicy: &deletePolicy,
				}); err != nil {
					return err
				}
				log.Info("Deleted deployment", "deployment", resourceToRollBack.name)

			case "job":
				err := clientset.BatchV1().Jobs(resourceToRollBack.namespace).Delete(context.Background(), resourceToRollBack.name, metav1.DeleteOptions{
					PropagationPolicy: &deletePolicy,
				})
				if err != nil {
					return err
				}
				log.Info("Deleted job", "service", resourceToRollBack.name)

			case "service":
				err := clientset.CoreV1().Services(resourceToRollBack.namespace).Delete(context.Background(), resourceToRollBack.name, metav1.DeleteOptions{
					PropagationPolicy: &deletePolicy,
				})
				if err != nil {
					return err
				}
				log.Info("Deleted service", "service", resourceToRollBack.name)

			default:
				log.Error("Unknown resource type to roll back", "resourceType", resourceToRollBack.resourceType)
			}
		}
	}
	return nil
}

func GetContainerLogs(log log.Logger, kubeClients []types.ContainerZone, p types.Project, c types.ContainerClaim) (logsForZones []LogsForZone, err error) {
	for _, client := range kubeClients {
		clientset := client.ClientSet

		podName := ""
		if c.RunType == "once" {
			podName = c.JobName()
		} else {
			podName = c.DeploymentName()
		}

		allPodsWithName, err := clientset.CoreV1().Pods(p.NamespaceName()).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("name=%s", podName)})
		if err != nil {
			return logsForZones, err
		}

		// should be only 1 pod with that name, but whatever
		for _, pod := range allPodsWithName.Items {
			req := clientset.CoreV1().Pods(p.NamespaceName()).GetLogs(pod.Name, &apiv1.PodLogOptions{
				Timestamps: true,
			})
			podLogs, err := req.Stream(context.Background()) // TODO: pass on this stream as a stream instead of doing all at once
			if err != nil {
				return logsForZones, err
			}
			defer podLogs.Close()

			buf := new(strings.Builder)
			_, err = io.Copy(buf, podLogs)
			if err != nil {
				return logsForZones, err
			}
			str := buf.String()

			logsForZones = append(logsForZones, LogsForZone{Zone: client.Name, Logs: strings.Split(str, "\n")})
		}
	}

	return logsForZones, nil
}

func GetContainerInstances(kubeClients []types.ContainerZone, containerName string) (containers []types.ContainerClaim, err error) {
	// TODO: user's own shit
	namespace := "default"
	deployment := containerName

	for _, client := range kubeClients {
		clientset := client.ClientSet
		deploymentDetails, err := clientset.AppsV1().Deployments(namespace).Get(context.Background(), deployment, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			fmt.Printf("Deployment %s in namespace %s not found\n", deployment, namespace)
			return containers, err
		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
			fmt.Printf("Error getting pod %s in namespace %s: %v\n",
				deployment, namespace, statusError.ErrStatus.Message)
			return containers, err
		} else if err != nil {
			return containers, err
		}

		fmt.Printf("Found deployment %s in namespace %s\n%s\n", deployment, namespace, deploymentDetails)
	}

	return containers, nil
}

func UpdateContainer() {
	// TODO
}

func DeleteContainer(log log.Logger, kubeClients []types.ContainerZone, project types.Project, containerClaim types.ContainerClaim, areWeRecreating bool) error {
	log.Debug("Deleting container", "container", containerClaim.Name)
	namespace := project.NamespaceName()

	for _, client := range kubeClients {
		if !slices.Contains(containerClaim.Zones, client.Name) {
			continue
		}
		clientset := client.ClientSet
		deletePolicy := metav1.DeletePropagationForeground

		for _, targetPort := range containerClaim.TargetPorts {
			log.Debug("Deleting service", "container", containerClaim.Name)
			err := clientset.CoreV1().Services(namespace).Delete(context.Background(), containerClaim.ServiceName(targetPort), metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			})
			if err != nil {
				return err
			}
		}

		if !areWeRecreating {
			log.Debug("Deleting secrets (env vars)", "container", containerClaim.Name)
			for _, envVarName := range containerClaim.EnvVarNames {
				if err := clientset.CoreV1().Secrets(namespace).Delete(context.TODO(), containerClaim.EnvVarSpecName(envVarName), metav1.DeleteOptions{
					PropagationPolicy: &deletePolicy,
				}); err != nil {
					return err
				}
			}
		} else {
			log.Debug("Not deleting secrets because we are recreating this container", "container", containerClaim.Name)
		}

		if containerClaim.RunType == "once" {
			log.Debug("Deleting job", "container", containerClaim.Name)
			err := clientset.BatchV1().Jobs(namespace).Delete(context.Background(), containerClaim.JobName(), metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			})
			if err != nil {
				return err
			}
		} else {
			deploymentsClient := clientset.AppsV1().Deployments(namespace)
			log.Debug("Deleting deployment", "container", containerClaim.Name)
			if err := deploymentsClient.Delete(context.Background(), containerClaim.DeploymentName(), metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func InitialiseKubeClients(notConnectedClients []types.ContainerZone) (connectedClients []types.ContainerZone, err error) {
	for _, c := range notConnectedClients {
		clientset, err := createKubeClient(c.Name)
		if err != nil {
			return nil, err
		}
		c.ClientSet = clientset
		connectedClients = append(connectedClients, c)
	}

	return connectedClients, err
}

func createKubeClient(clusterName string) (*kubernetes.Clientset, error) {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String(fmt.Sprintf("kubeconfig-%s", clusterName), filepath.Join(home, ".kube", fmt.Sprintf("%s.conf", clusterName)), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String(fmt.Sprintf("kubeconfig-%s", clusterName), "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func int32Ptr(i int32) *int32 { return &i }
