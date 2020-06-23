package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"../../../vsystemclient"
)

var (
	gBearerToken  string
	gConnectionID = "DI_DATA_LAKE"
	gEndpoint     string
	gTimeout      = 15 * time.Second
	gVsystemInfo  vsystemclient.VSystemInfo
)

var (
	flagUsingvSystem = true
	flagUsingTLSCA   = true
)

var ( //deprecated
	flagCheckRuntimeStorage = true
	flagUsingFileStorage    = false
)

var Log func(string)
var Logf func(string, ...interface{})
var Errorf func(string, ...interface{})

var OutArtifact func(interface{})
var OutPath func(interface{})
var OutError func(interface{})

var GetGraphHandle func() string
var GetString func(string) string

var (
	flagUsingInternalConnection = true
	internalClient              *http.Client
)

func Setup() {
	if flagUsingvSystem {
		vsystemInfo, err := vsystemclient.InitvSystemInfo(gEndpoint)
		if nil != err {
			ProcessErrorSetup("Auth", err)
			return
		}
		gVsystemInfo = *vsystemInfo
		Logf("ArtifactProducer: using the following endpoint with BearerToken: %v", gVsystemInfo)

		client, err := vsystemclient.CreateInternalClient(&vsystemclient.CertFromConnManager{Info: gVsystemInfo})
		if nil != err {
			ProcessErrorSetup("createInternalClient", err)
			return
		}
		internalClient = client
	}
	if flagCheckRuntimeStorage && flagUsingFileStorage {
		runtimeStoragePath, ok := os.LookupEnv("VSYSTEM_RUNTIME_STORE_MOUNT_PATH")
		if !ok {
			ProcessErrorSetup("runtimeStorage", errors.New("runtimeStorage via $VSYSTEM_RUNTIME_STORE_MOUNT_PATH not found"))
			return
		}
		if _, err := os.Stat(runtimeStoragePath); os.IsNotExist(err) {
			ProcessErrorSetup("runtimeStoragePath", err)
			return
		}
		Logf("ArtifactProducer: using the following runtimeStorage: %q", runtimeStoragePath)
	}
}

///////////////////////

type ArtifactPostResponseData struct {
	ID      string
	Name    string
	Status  string
	URI     string
	Message string
}

func GetAPIArtifactRequest(endpoint string, expectedStatusCode int, bearerToken string) (result *ArtifactPostResponseData, jsonResponse *string, err error) {

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if nil != err {
		return nil, nil, err
	}

	req.Header.Set("Authorization", bearerToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	ctx, cncl := context.WithTimeout(context.Background(), gTimeout)
	defer cncl()
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if nil != err {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatusCode {
		var addErrorMsg string
		if resp.StatusCode == 400 {
			addErrorMsg = "\n\tMake sure that the graph execution was scheduled by the ML Operations Cockpit."
		}
		return nil, nil, fmt.Errorf("endpoint: %v status:%v : %v", endpoint, resp.Status, addErrorMsg)
	}

	jsonBody, err := ioutil.ReadAll(resp.Body)
	if nil != err {
		return nil, nil, err
	}

	var tmp ArtifactPostResponseData
	if err := json.Unmarshal(jsonBody, &tmp); nil != err {
		return nil, nil, err
	}
	if len(tmp.Name) == 0 {
		return nil, nil, errors.New("mandatory MLApi.name is empty")
	}
	if len(tmp.URI) == 0 {
		return nil, nil, errors.New("mandatory MLApi.uri is empty")
	}

	str := string(jsonBody)
	return &tmp, &str, nil
}

func GetArtifact(filepath string) (content []byte, err error) {
	content, err = ioutil.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("Error opening Artifact: %q %v", filepath, err)
	}
	return content, nil
}

////////////////////////

func GetPath(uri string) (protocol string, path *string, err error) {
	path = new(string)
	if strings.Contains(uri, "://") {
		token := strings.Split(uri, "://")
		if strings.Contains(token[0], "file") {
			*path = token[1]
			protocol = token[0]
		} else if strings.Contains(token[0], "dh-dl") {
			*path = token[1]
			protocol = token[0]
		} else {
			return "", nil, fmt.Errorf("unspecified protocol %q", token[0])
		}
	} else {
		*path = uri
	}
	return protocol, path, nil
}

func inPortToString(val interface{}) (*string, error) {
	artifactID, ok := val.(string)
	if !ok {
		return nil, errors.New("Type of inPort is not a string")
	}
	if len(artifactID) == 0 {
		return nil, errors.New("inPort is empty")
	}
	return &artifactID, nil
}

func ConsumeArtifact(openFileURL, authToken string) error {
	consumeFileURL := fmt.Sprintf("%v?op=OPEN", openFileURL)
	Logf("ArtifactConsumer: hdfs command: %v", consumeFileURL)

	if nil == internalClient {
		return errors.New("internalClient is not initialized")
	}

	req, _ := http.NewRequest("GET", consumeFileURL, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", authToken))
	res, err := getClient(internalClient).Do(req)
	if nil != err {
		return err
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if nil != err {
		return err
	}
	Log("ArtifactConsumer: sending artifact to OutArtifact")
	OutArtifact(body)
	return nil
}

func getNextPaths(baseURL *url.URL, authToken string, nextArtifactStatus []string) ([]string, error) {
	currentURL := baseURL.String()
	getFileStatusURL := fmt.Sprintf("%v?op=LISTSTATUS", currentURL)
	Logf("ArtifactConsumer: hdfs get file status command: %v", getFileStatusURL)

	req, _ := http.NewRequest("GET", getFileStatusURL, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", authToken))
	res, err := getClient(internalClient).Do(req)
	if nil != err {
		return nil, err
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if nil != err {
		return nil, err
	}

	type SDLFileStatus struct {
		PathSuffix       string
		Type             string
		Length           int
		Owner            string
		Group            string
		Permission       string
		AccessTime       uint64
		ModificationTime uint64
		BlockSize        uint64
		Replication      int
	}

	type SDLStatuses struct {
		FileStatuses struct {
			FileStatus []SDLFileStatus
		}
	}

	var status SDLStatuses
	err = json.Unmarshal(body, &status)
	if nil != err {
		return nil, err
	}

	Logf("ArtifactConsumer: file at %v status: %v", currentURL, status)

	currentPath := baseURL.Path
	for _, stat := range status.FileStatuses.FileStatus {
		artifactURL := path.Join(currentPath, stat.PathSuffix)
		if stat.Type == "DIRECTORY" {
			nextArtifactStatus = append(nextArtifactStatus, artifactURL)
		} else if stat.Type == "FILE" {
			baseURL.Path = artifactURL
			if err = ConsumeArtifact(baseURL.String(), authToken); err != nil {
				return nil, err
			}
		}
	}

	return nextArtifactStatus, nil
}

func getFileStatus(baseURL *url.URL, authToken string) (string, error) {
	currentURI := baseURL
	currentURI.RawQuery = ""
	currentURL := currentURI.String()
	//getFileStatusURL := fmt.Sprintf("%v?op=LISTSTATUS", currentURL)
	getFileStatusURL := fmt.Sprintf("%v?op=GETFILESTATUS", currentURL)
	Logf("ArtifactConsumer: hdfs get file status command: %v", getFileStatusURL)

	req, _ := http.NewRequest("GET", getFileStatusURL, nil)
	req.Header.Add("Authorization", authToken)
	res, err := getClient(internalClient).Do(req)
	if nil != err {
		return "", err
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if nil != err {
		return "", err
	}

	if res.StatusCode != 200 {
		return "", fmt.Errorf("response status from DataLakeEndpoint %q: %q", res.Status, body)
	}

	Logf("response status from DataLakeEndpoint %q: %q", res.Status, body)

	type SDLFileStatus struct {
		PathSuffix       string
		Type             string
		Length           int
		Owner            string
		Group            string
		Permission       string
		AccessTime       uint64
		ModificationTime uint64
		BlockSize        uint64
		Replication      int
	}

	type SDLStatuses struct {
		FileStatus SDLFileStatus
	}

	var status SDLStatuses
	err = json.Unmarshal(body, &status)
	if nil != err {
		return "", err
	}

	Logf("ArtifactConsumer: file at %v status: %v", currentURL, status)
	return status.FileStatus.Type, nil
}

func Walk(baseDirURL *url.URL, authToken string) error {
	nextArtifactStatuses := make([]string, 0)
	nextArtifactStatuses, err := getNextPaths(baseDirURL, authToken, nextArtifactStatuses)
	if err != nil {
		return err
	}
	for len(nextArtifactStatuses) > 0 {
		nextArtifactStatus := nextArtifactStatuses[0]
		nextArtifactStatuses = nextArtifactStatuses[1:]
		baseDirURL.Path = nextArtifactStatus
		nextArtifactStatuses, err = getNextPaths(baseDirURL, authToken, nextArtifactStatuses)
		if err != nil {
			return err
		}
	}
	return nil
}

// call with: receiveFileInput(filePathPtr, datafilePath, m.URI)
func receiveFileInput(filePathPtr *string, datafilePath string, URI string) []byte {
	*filePathPtr = strings.Replace(*filePathPtr, gConnectionID+"/", "", -1)
	u, err := url.Parse(datafilePath)
	if err != nil {
		ProcessErrorInArtifact("Parse data url", err)
		return []byte{}
	}
	u.Path = path.Join(u.Path, *filePathPtr)
	Logf("ArtifactConsumer: Full Path to use: %q", URI)
	outArtifactBlob := []byte(URI)
	return outArtifactBlob
}

// call with: showAllSDLFilesInFolder(datafilePath, *filePathPtr, dataLakeCon)
func showAllSDLFilesInFolder(datafilePath string, filePathPtr *string, dataLakeCon vsystemclient.DLContentData) {
	getFileStatusURL := fmt.Sprintf("%v/%v?op=LISTSTATUS", datafilePath, *filePathPtr)
	Logf("ArtifactConsumer: hdfs get file status command: %v", getFileStatusURL)

	req, _ := http.NewRequest("GET", getFileStatusURL, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", dataLakeCon.AuthToken))
	res, err := getClient(internalClient).Do(req)
	if nil != err {
		ProcessErrorInArtifact("httpClient", err)
		return
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if nil != err {
		ProcessErrorInArtifact("httpClient", err)
		return
	}

	type SDLFileStatus struct {
		PathSuffix       string
		Type             string
		Length           int
		Owner            string
		Group            string
		Permission       string
		AccessTime       uint64
		ModificationTime uint64
		BlockSize        uint64
		Replication      int
	}

	type SDLStatuses struct {
		FileStatuses struct {
			FileStatus []SDLFileStatus
		}
	}

	var status SDLStatuses
	err = json.Unmarshal(body, &status)
	if nil != err {
		ProcessErrorInArtifact("internalClient", err)
		return
	}

	Logf("ArtifactConsumer: file at %v status: %v", *filePathPtr, status)

	for _, stat := range status.FileStatuses.FileStatus {
		Logf("ArtifactConsumer: file at %v type: %v", *filePathPtr, stat)
	}
}

func InArtifactID(val interface{}) {
	var (
		inArtifactID string
		inAPIVersion string
	)

	artifactIDPtr, err := inPortToString(val)
	if nil != err {
		ProcessErrorInArtifact("inPortToString", err)
		return
	}
	inArtifactID = *artifactIDPtr

	Log(fmt.Sprintf("ArtifactConsumer: started with executionID: %v", GetGraphHandle()))

	if err := CheckMandatoryParameter(&inAPIVersion, "apiVersion", GetString); nil != err {
		ProcessErrorInArtifact("parameter", err)
		return
	}
	artifactsEndpoint := CreateArtifactEndpoint(gVsystemInfo.Endpoint, inAPIVersion, "artifacts/"+inArtifactID)
	m, jsonBody, err := GetAPIArtifactRequest(artifactsEndpoint, 200, gVsystemInfo.BearerToken)
	if nil != err {
		ProcessErrorInArtifact("GetAPIArtifactRequest", err)
		return
	}

	if nil == m {
		ProcessErrorInArtifact("GetAPIArtifactRequest", errors.New("artifact response message is empty"))
		return
	}

	Logf("ArtifactConsumer: GET Response: %v", *jsonBody)

	protocol, filePathPtr, err := GetPath(m.URI)
	if nil != err {
		ProcessErrorInArtifact("GetPath", err)
		return
	}

	Logf("ArtifactConsumer: using path: %v %v", protocol, *filePathPtr)
	switch protocol {
	case "dh-dl":
		connectionManagerURL := vsystemclient.CreateConnectionEndpoint(gVsystemInfo.Endpoint)
		DataLakeEndpoint := vsystemclient.CreateDataLakeEndpoint(connectionManagerURL, gConnectionID)

		Logf("ArtifactConsumer: DataLakeEndpoint: %v", DataLakeEndpoint)
		contentData, _, err := vsystemclient.ReceiveDatalakeInformation(DataLakeEndpoint, gVsystemInfo.BearerToken, gVsystemInfo.DatahubTenant, gVsystemInfo.DatahubUser)
		if nil != err {
			ProcessErrorInArtifact("ReceiveDatalakeInformation", err)
			return
		}

		dataLakeCon := contentData.DLData
		requestSDL, err := executeOpeningFileOnSDL(filePathPtr, dataLakeCon)
		if nil != err {
			ProcessErrorInArtifact("requestSDL", err)
			return
		}

		Logf("ArtifactConsumer: hdfs command: %v", requestSDL.endpoint)
		u, err := url.Parse(requestSDL.endpoint)
		if err != nil {
			ProcessErrorInArtifact("Parse data url", err)
			return
		}
		status, err := getFileStatus(u, requestSDL.bearerToken)
		if err != nil {
			ProcessErrorInArtifact("getNextPaths", err)
			return
		}

		switch status {
		case "FILE":
			executeSDLFile(requestSDL)
		case "DIRECTORY":
			executeSDLDirectory(*filePathPtr)
		}

	case "file":
		executeRunStorageFile(*filePathPtr)
	}

}

func executeSDLFile(requestSDL *requestSDLInfos) {
	body, err := executeSDLRequestWithHDFS(requestSDL)
	if nil != err {
		ProcessErrorInArtifact("executeSDLRequestWithHDFS", err)
		return
	}
	Log("ArtifactConsumer: sending artifact to OutArtifact")
	OutArtifact(body)
}

func executeSDLDirectory(URI string) {
	Logf("ArtifactConsumer: DIRECTORY TODO %q", "")

	msg := createPathMessage(URI)
	Logf("ArtifactConsumer: sending path to OutPath: %v", msg)
	vsystemclient.TrySendingToPort(OutPath, msg, gOperatorConfig.OperatorName, Logf)
}

func createPathMessage(URI string) interface{} {
	headers := make(map[string]interface{}, 3)
	headers["version"] = "v1"
	headers["protocol"] = "dh-dl"
	headers["connectionID"] = gConnectionID

	message := make(map[string]interface{}, 2)
	message["Attributes"] = headers
	message["Body"] = URI
	message["Encoding"] = "string"
	return message
}

func executeRunStorageFile(filePath string) {
	artifact, err := ioutil.ReadFile(filePath)
	if nil != err {
		ProcessErrorInArtifact("Read Artifact", err)
		return
	}
	Log("ArtifactConsumer: sending artifact to OutArtifact")
	OutArtifact(artifact)
}

func executeOpeningFileOnSDL(filePathPtr *string, dataLakeCon vsystemclient.DLContentData) (*requestSDLInfos, error) {
	*filePathPtr = strings.Replace(*filePathPtr, gConnectionID+"/", "", -1)

	con := vsystemclient.CreateDataLakeConnectionEndpoint(dataLakeCon)
	Logf("ArtifactConsumer: connection: %v with internal: %v and CA: %v", con, flagUsingInternalConnection, flagUsingTLSCA)
	datafilePath := vsystemclient.CreateWebHdfsURI(con, flagUsingInternalConnection)
	Logf("ArtifactConsumer: path to file: %v", datafilePath)
	openFileURL := fmt.Sprintf("%v/%v?op=OPEN", datafilePath, *filePathPtr)
	return &requestSDLInfos{
		httpMethod:         http.MethodGet,
		endpoint:           openFileURL,
		bearerToken:        fmt.Sprintf("Bearer %v", dataLakeCon.AuthToken),
		expectedStatusCode: http.StatusCreated,
	}, nil
}

func executeSDLRequestWithHDFS(meta *requestSDLInfos) ([]byte, error) {
	// create file and write file

	req, err := http.NewRequest(meta.httpMethod, meta.endpoint, meta.payload)
	if nil != err {
		return nil, fmt.Errorf("http request creation error: %v", err)
	}

	req.Header.Add("Authorization", meta.bearerToken)

	if nil == internalClient {
		return nil, errors.New("internalClient is not initialized")
	}

	res, err := getClient(internalClient).Do(req)
	if nil != err {
		return nil, fmt.Errorf("hdfs create: %v", err)
	}

	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if nil != err {
		return nil, fmt.Errorf("httpClient: %v", err)
	}

	return body, nil
}

func InArtifact(val interface{}) {
	Log("ArtifactConsumer: InArtifact called")

	message, ok := val.(map[string]interface{})
	if !ok {
		ProcessErrorInArtifact("message", errors.New("convert error"))
		return
	}
	messageHeader, ok := message["Attributes"]
	if !ok {
		ProcessErrorInArtifact("message", errors.New("Attributes is not found in inMessage"))
		return
	}

	attributes, ok := messageHeader.(map[string]interface{})
	if !ok {
		ProcessErrorInArtifact("message", errors.New("convert error"))

		return
	}
	artifactID, ok := attributes["artifactID"]
	if !ok {
		ProcessErrorInArtifact("message", errors.New("artifactID is not found in attributes"))
		return
	}

	Log(fmt.Sprintf("ArtifactConsumer: inArtifact called with artifactID: %v", artifactID))
	InArtifactID(artifactID)
}

func CheckMandatoryParameter(mandatoryValue *string, key string, getFunc func(string) string) error {
	value := getFunc(key)
	if len(value) == 0 {
		return fmt.Errorf("mandatory parameter %q is not set", key)
	}
	*mandatoryValue = value
	return nil
}

func CreateArtifactEndpoint(prefix string, path string, suffix string) string {
	endpoint := fmt.Sprintf("%v/app/ml-api/api/%v/%v", prefix, path, suffix)
	return endpoint
}

func getClient(internalClient *http.Client) *http.Client {
	if flagUsingInternalConnection {
		return internalClient
	}
	return http.DefaultClient
}

func ProcessErrorSetup(operation string, err error) {
	gOperatorConfig.Operation = operation
	vsystemclient.ProcessError(err, Errorf, OutError, gOperatorConfig, GetString("processName"), "Setup")
}

func ProcessErrorInArtifact(operation string, err error) {
	gOperatorConfig.Operation = operation
	vsystemclient.ProcessError(err, Errorf, OutError, gOperatorConfig, GetString("processName"), "InArtifact")
}

var gOperatorConfig = vsystemclient.OperatorConfig{
	OperatorName: "ArtifactConsumer",
	Operation:    "artifact consumption",
	OperatorPath: "com.sap.ml.artifact.consumer",
}

type requestSDLInfos struct {
	httpMethod         string
	endpoint           string
	bearerToken        string
	payload            io.Reader
	contentType        string
	expectedStatusCode int
}
