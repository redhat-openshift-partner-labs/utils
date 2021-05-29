package utils

import (
	"context"
	"encoding/json"
	"github.com/google/go-github/v33/github"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/sheets/v4"
	"io/ioutil"
	. "k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	"os"
)

func GithubAuthenticate() (*github.Client, context.Context) {
	accesstoken := os.Getenv("GITHUB_TOKEN")
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: accesstoken},
	)
	tc := oauth2.NewClient(ctx, ts)
	gc := github.NewClient(tc)
	return gc, ctx
}

func K8sAuthenticate() *kubernetes.Clientset {
	// create k8s client
	cfg, err := clientcmd.BuildConfigFromFlags("", os.Getenv("OPENSHIFT_KUBECONFIG"))
	ErrorCheck("The kubeconfig could not be loaded", err)
	clientset, err := kubernetes.NewForConfig(cfg)

	return clientset
}

func DefaultClientK8sAuthenticate() (*rest.Config, error) {
	cfg, err := clientcmd.LoadFromFile(os.Getenv("OPENSHIFT_KUBECONFIG"))
	ErrorCheck("The kubeconfig could not be loaded", err)
	dc := clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{})

	return dc.ClientConfig()
}

func DynamicClientK8sAuthenticate() (Interface, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", os.Getenv("OPENSHIFT_KUBECONFIG"))
	ErrorCheck("The kubeconfig could not be loaded", err)
	dc, err := NewForConfig(cfg)

	return dc, err
}

func GoogleDriveAuthenticate(credentials string, token string) (client *http.Client, err error) {
	credentialsFileBytes, err := ioutil.ReadFile(credentials)
	ErrorCheck("Unable to read credentials file: ", err)

	credentialsConfig, err := google.ConfigFromJSON(credentialsFileBytes,
		drive.DriveScope, drive.DriveFileScope, sheets.DriveScope, sheets.SpreadsheetsScope)
	ErrorCheck("Unable to create config from credentials file: ", err)

	tokenFile, err := os.Open(token)
	ErrorCheck("Unable to open token file: ", err)
	defer func(tokenFile *os.File) {
		err := tokenFile.Close()
		ErrorCheck("Unable to close token file: ", err)
	}(tokenFile)

	tokenJSON := &oauth2.Token{}
	err = json.NewDecoder(tokenFile).Decode(tokenJSON)
	ErrorCheck("Unable to parse token file: ", err)

	credentialsClient := credentialsConfig.Client(context.Background(), tokenJSON)

	return credentialsClient, nil
}
