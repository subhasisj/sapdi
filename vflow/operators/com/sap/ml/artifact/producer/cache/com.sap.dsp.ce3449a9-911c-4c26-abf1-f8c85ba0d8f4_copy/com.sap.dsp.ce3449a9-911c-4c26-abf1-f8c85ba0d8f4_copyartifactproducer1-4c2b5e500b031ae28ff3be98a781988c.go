package main

import (
	"bytes"
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
	"sync"
	"time"

	"../../../vsystemclient"
)

var (
	gBearerToken   string
	gDatahubUser   string
	gDatahubTenant string
	gConnectionID  = "DI_DATA_LAKE"
	gEndpoint      string
	gTimeout       = 15 * time.Second
	gVsystemInfo   vsystemclient.VSystemInfo
)

var (
	flagUsingvSystem = true
)

var ( //deprecated
	flagUsingTLSCA          = true
	flagCheckRuntimeStorage = true
	flagUsingFileStorage    = false
	gRuntimeStoragePath     = "/tenant/runtime"
)

var Log func(string)
var Logf func(string, ...interface{})
var Errorf func(string, ...interface{})

var OutArtifactId func(interface{})
var OutArtifact func(interface{})
var OutError func(interface{})

var GetGraphHandle func() string
var GetString func(string) string

var internalClient *http.Client

type ArtifactPostRequestMetaData struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	URI         string `json:"uri"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	ExecutionID string `json:"executionId"`
}

var once sync.Once
var lock = &sync.Mutex{}
var counter *uint64

func GetSingletonCounter() uint64 {
	once.Do(func() {
		counter = new(uint64)
	})

	lock.Lock()
	defer lock.Unlock()

	activeCounter := *counter
	*counter++
	return activeCounter
}

func InPath(val interface{}) {

	path, ok := val.(string)
	if !ok {
		ProcessErrorInArtifact("Path", errors.New(""))
		return
	}

	Log("ArtifactProducer: creating metadata for artifact production")
	metaData, artifactConf, operatorConf, err := createExistingArtifactRequestMetaDataNew(GetString, GetGraphHandle(), path)
	if nil != err {
		ProcessErrorInArtifact("CreateMetadata", err)
		return
	}

	Logf("ArtifactProducer: checking path on SDL")
	metaDataURI, _, err := ProduceArtifactInMode(executeCheckingOfFileStatus, []byte{}, *metaData, artifactConf.endpoint, operatorConf.service)
	if nil != err {
		ProcessErrorInArtifact("ProduceArtifact", err)
		return
	}

	Logf("ArtifactProducer: registering artifact to the following endpoint: %q", artifactConf.endpoint)
	_, responseMetadata, err := registerArtifact(metaData, *metaDataURI, artifactConf.endpoint)
	if nil != err {
		ProcessErrorInArtifact("RegisterArtifact", err)
		return
	}

	message := CreateMessage(responseMetadata.ID, metaData.Name, metaData.Kind)
	vsystemclient.TrySendingToPort(OutArtifact, message, gOperatorConfig.OperatorName, Logf)
}

func InArtifact(val interface{}) {
	artifact, ok := val.([]byte)
	if !ok {
		ProcessErrorInArtifact("Conversion", errors.New("message body cannot be converted to []byte"))
		return
	}
	Logf("ArtifactProducer: called InArtifact with length: %q", len(artifact))

	Log("ArtifactProducer: creating metadata for artifact production")
	metaData, artifactConf, operatorConf, err := createNewArtifactRequestMetaDataNew(GetString, GetGraphHandle(), GetSingletonCounter())
	if nil != err {
		ProcessErrorInArtifact("CreateMetadata", err)
		return
	}

	Logf("ArtifactProducer: producing file on SDL")
	metaDataURI, _, err := ProduceArtifactInMode(executeCreatingFileOnSDLNew, artifact, *metaData, artifactConf.endpoint, operatorConf.service)
	if nil != err {
		ProcessErrorInArtifact("ProduceArtifact", err)
		return
	}

	Logf("ArtifactProducer: registering artifact to the following endpoint: %q", artifactConf.endpoint)
	_, responseMetadata, err := registerArtifact(metaData, *metaDataURI, artifactConf.endpoint)
	if nil != err {
		ProcessErrorInArtifact("RegisterArtifact", err)
		return
	}

	message := CreateMessage(responseMetadata.ID, metaData.Name, metaData.Kind)
	vsystemclient.TrySendingToPort(OutArtifact, message, gOperatorConfig.OperatorName, Logf)
}

func CreateMessage(artifactID string, artifactName string, artifactKind string) interface{} {
	headers := make(map[string]interface{}, 3)
	headers["artifactID"] = artifactID
	headers["name"] = artifactName
	headers["kind"] = artifactKind

	message := make(map[string]interface{}, 2)
	message["Attributes"] = headers
	message["Body"] = ""
	return message
}

func Setup() {
	if flagUsingvSystem {
		vsystemInfo, err := vsystemclient.InitvSystemInfo(gEndpoint)
		if nil != err {
			ProcessErrorSetup("Auth", err)
			return
		}
		gVsystemInfo = *vsystemInfo
		gBearerToken = vsystemInfo.BearerToken
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

type MLAPIMessage struct {
	ID      string
	Name    string
	Status  string
	URI     string
	Message string
}

func GetAPIArtifactRequest(httpMethod string, expectedStatusCode http.ConnState, endpoint string, bearerToken string, reader io.Reader) (result *MLAPIMessage, jsonResponse *[]byte, err error) {

	req, err := http.NewRequest(httpMethod, endpoint, reader)
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

	if resp.StatusCode != int(expectedStatusCode) {
		var addErrorMsg string
		if resp.StatusCode == http.StatusBadRequest {
			addErrorMsg = "\n\tMake sure that the pipeline is registered in the ML Scenario Manager"
		}
		return nil, nil, errors.New("Error: " + resp.Status + addErrorMsg)
	}

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil, nil
	}

	jsonBody, err := ioutil.ReadAll(resp.Body)
	if nil != err {
		return nil, nil, err
	}

	if len(jsonBody) == 0 {
		return nil, nil, errors.New("JSON response should not be empty")
	}

	var tmp MLAPIMessage
	if err := json.Unmarshal(jsonBody, &tmp); nil != err {
		return nil, nil, err
	}

	return &tmp, &jsonBody, nil
}

func GetPath(uri string) (path string, err error) {
	// assumption: uri= <protocol>:/<path>
	if strings.Contains(uri, "://") {
		uriPart := strings.Split(uri, "://")
		if !strings.Contains(uriPart[0], "file") {
			return "", fmt.Errorf("unspecified protocol %q", uriPart[0])
		}
		return uriPart[1], nil
	}
	return uri, nil
}

func CreateFile(filePath string, content []byte) (rootPath string, err error) {
	const pathStats os.FileMode = 0755
	const fileStats os.FileMode = 0644

	rootPath, _ = path.Split(filePath)
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		if err = os.MkdirAll(rootPath, pathStats); err != nil {
			return "", err
		}
	}
	if err = ioutil.WriteFile(filePath, content, fileStats); nil != err {
		return "", err
	}
	return rootPath, nil
}

func ProduceArtifactInMode(modeFunction func(string, string) (*requestSDLInfos, error), val []byte, metaData ArtifactPostRequestMetaData, artifactsEndpoint string, confService string) (metaDataURI *string, result *MLAPIMessage, err error) {
	Logf("ArtifactProducer: starting producer with service %q and runtimeID %q", confService, GetGraphHandle())
	Logf("ArtifactProducer: incoming blob with length: %q", len(val))

	artifactURI := metaData.URI

	switch confService {
	case "SDL":
		connectionManagerURL := vsystemclient.CreateConnectionEndpoint(gVsystemInfo.Endpoint)
		DataLakeEndpoint := vsystemclient.CreateDataLakeEndpoint(connectionManagerURL, gConnectionID)
		datalakeURL := createDataLakePath(artifactURI)
		Logf("ArtifactProducer: created datalakeURL: %q", datalakeURL)

		contentData, _, err := vsystemclient.ReceiveDatalakeInformation(DataLakeEndpoint, gVsystemInfo.BearerToken, gVsystemInfo.DatahubTenant, gVsystemInfo.DatahubUser)
		if nil != err {
			return nil, nil, err
		}
		dataLakeCon := contentData.DLData

		con := createDataLakeConnectionEndpoint(dataLakeCon)
		Logf("ArtifactProducer: connection: %v, using CA: %v", con, flagUsingTLSCA)
		requestSDL, err := modeFunction(con.internalCon, datalakeURL)
		if nil != err {
			return nil, nil, err
		}
		requestSDL.bearerToken = fmt.Sprintf("Bearer %v", dataLakeCon.AuthToken)
		requestSDL.payload = bytes.NewReader(val)

		Logf("ArtifactProducer: hdfs command: %v", requestSDL.endpoint)
		err = executeSDLRequestWithHDFS(requestSDL)
		if nil != err {
			return nil, nil, err
		}

		artifactURI = createArtifactFullURI(gConnectionID, datalakeURL)

	case "File":
		if !flagUsingFileStorage {
			return nil, nil, fmt.Errorf("service %q is not supported anymore", confService)
		}

		fileURI := fmt.Sprintf("file://%v/%v", gRuntimeStoragePath, artifactURI)
		filePath, err := GetPath(fileURI)
		if nil != err {
			return nil, nil, err
		}
		if _, err := CreateFile(filePath, val); err != nil {
			return nil, nil, err
		}
		Logf("ArtifactProducer: Artifact written to %q", filePath)
		artifactURI = fileURI

	default:
		return nil, nil, fmt.Errorf("ArtifactProducer: service %q is not specified, stopping function", confService)
	}

	Logf("ArtifactProducer: URI created %q", artifactURI)
	return &artifactURI, nil, nil
}

func registerArtifact(metaData *ArtifactPostRequestMetaData, URI string, artifactsEndpoint string) (*string, *MLAPIMessage, error) {

	metaData.URI = URI
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&metaData); err != nil {
		return nil, nil, err
	}

	Logf("ArtifactProducer: writing to endpoint %q", artifactsEndpoint)
	Logf("ArtifactProducer: using request json %q", &buf)
	m, jsonBody, err := GetAPIArtifactRequest(http.MethodPost, 201, artifactsEndpoint, gVsystemInfo.BearerToken, &buf)
	if nil != err {
		return nil, nil, fmt.Errorf("PostApiArtifact: %v", err)
	}

	Logf("ArtifactProducer: response received : %q", *jsonBody)
	Logf("ArtifactProducer: artifact registered with ID %q", m.ID)
	return &m.ID, m, nil
}

func executeCreatingFileOnSDLNew(connection string, datalakeURL string) (*requestSDLInfos, error) {
	fileURLPath := createURIForSDL(connection, datalakeURL)
	createFileURL := createURIForCreatingFileOnSDL(fileURLPath)
	return &requestSDLInfos{
		httpMethod:         http.MethodPut,
		endpoint:           createFileURL,
		contentType:        "application/octet-stream",
		expectedStatusCode: http.StatusCreated,
	}, nil
}

func executeCheckingOfFileStatus(connection string, datalakeURL string) (*requestSDLInfos, error) {
	u, err := url.Parse(connection)
	if err != nil {
		return nil, fmt.Errorf("connection URL parse error: %v", err)
	}
	u.Path = path.Join(u.Path, "webhdfs", "v1", datalakeURL)

	fileURLPath := u.String()
	getFileStatusURL := fmt.Sprintf("%v?op=GETFILESTATUS", fileURLPath)
	return &requestSDLInfos{
		httpMethod:         http.MethodGet,
		endpoint:           getFileStatusURL,
		contentType:        "application/json",
		expectedStatusCode: http.StatusOK,
	}, nil
}

func CheckMandatoryParameter(mandatoryValue *string, key string, getFunc func(string) string) error {
	value := getFunc(key)
	if len(value) == 0 {
		return fmt.Errorf("mandatory parameter %q is not set", key)
	}
	*mandatoryValue = value
	return nil
}

func CheckArtifactKind(artifactKind string) (err error) {
	switch artifactKind {
	case
		"model",
		"dataset",
		"other":
		return nil
	}
	return fmt.Errorf("artifactKind should be \"{model,dataset,other}\", got %q", artifactKind)
}

func CreateArtifactEndpoint(prefix string, path string, suffix string) string {
	endpoint := fmt.Sprintf("%v/app/ml-api/api/%v/%v", prefix, path, suffix)
	return endpoint
}

func createDataLakePath(path string) string {
	if strings.HasPrefix(path, "/") {
		path = strings.TrimLeft(path, "/")
	}
	return path
}

func createArtifactFullURI(connectionID string, path string) string {
	if strings.HasPrefix(path, "/") {
		path = strings.TrimLeft(path, "/")
	}
	return fmt.Sprintf("dh-dl://%v/%v", connectionID, path)
}

func createDataLakeConnectionEndpoint(dataLakeCon vsystemclient.DLContentData) Connection {
	var protocol string
	switch dataLakeCon.Protocol {
	case "webhdfs":
		protocol = "http"
	case "swebhdfs":
		protocol = "https"
	}
	return Connection{
		internalCon: fmt.Sprintf("%v://%v:%v", protocol, dataLakeCon.Host, dataLakeCon.Port),
		externalCon: fmt.Sprintf("%v://%v:%v", protocol, dataLakeCon.PublicHost, dataLakeCon.PublicPort),
	}
}

func createURIForSDL(endpoint string, URL string) string {
	return fmt.Sprintf("%v/webhdfs/v1/%v", endpoint, URL)
}

func createURIForCreatingFileOnSDL(uri string) string {
	return fmt.Sprintf("%v?op=CREATE&overwrite=false", uri)
}

func executeSDLRequestWithHDFS(meta *requestSDLInfos) error {
	// create file and write file

	req, err := http.NewRequest(meta.httpMethod, meta.endpoint, meta.payload)
	if nil != err {
		return fmt.Errorf("http request creation error: %v", err)
	}
	req.Close = true

	req.Header.Add("Authorization", meta.bearerToken)
	req.Header.Add("Content-Type", meta.contentType)

	if nil == internalClient {
		return errors.New("internalClient is not initialized")
	}

	res, err := internalClient.Do(req)
	if nil != err {
		return fmt.Errorf("hdfs create: %v", err)
	}

	io.Copy(ioutil.Discard, res.Body)
	defer res.Body.Close()

	if res.StatusCode != int(meta.expectedStatusCode) {
		return fmt.Errorf("response status %q for request %v : %v", res.Status, meta.httpMethod, meta.endpoint)
	}

	return nil
}

func createExistingArtifactRequestMetaDataNew(getFunction func(string) string, executionID string, path string) (*ArtifactPostRequestMetaData, *artifactConfigs, *operatorConfigs, error) {
	return createArtifactRequestMetaData(getFunction, executionID, 0, path)
}

func createNewArtifactRequestMetaDataNew(getFunction func(string) string, executionID string, counterSuffix uint64) (*ArtifactPostRequestMetaData, *artifactConfigs, *operatorConfigs, error) {
	return createArtifactRequestMetaData(getFunction, executionID, counterSuffix, "")
}

func generateArtifactConfiguration(getFunction func(string) string) (*artifactConfigs, *operatorConfigs, error) {
	var (
		inAPIVersion     string
		inOperatorName   string
		inService        string
		inVersionControl string

		inArtifactName        string
		inArtifactKind        string
		inArtifactDescription string
		inArtifactPrefix      string
		inArtifactSuffix      string
	)

	const (
		currentAPIVersion = "v1"
	)

	if err := CheckMandatoryParameter(&inAPIVersion, "apiVersion", GetString); nil != err {
		return nil, nil, err
	}
	if err := CheckMandatoryParameter(&inOperatorName, "processName", GetString); nil != err {
		return nil, nil, err
	}
	if err := CheckMandatoryParameter(&inService, "service", GetString); nil != err {
		return nil, nil, err
	}
	if err := CheckMandatoryParameter(&inVersionControl, "versionControl", GetString); nil != err {
		return nil, nil, err
	}

	switch inVersionControl {
	case "conf", "auto":
		if err := CheckMandatoryParameter(&inArtifactName, "artifactName", GetString); nil != err {
			return nil, nil, errors.New("mandatory parameter \"artifactName\" is not set")
		}
		if err := CheckMandatoryParameter(&inArtifactKind, "artifactKind", GetString); nil != err {
			return nil, nil, errors.New("mandatory parameter \"artifactKind\" is not set")
		}
		inArtifactDescription = GetString("description")
		inArtifactPrefix = GetString("prefix")
		inArtifactSuffix = GetString("suffix")
	case "input":
		if err := CheckMandatoryParameter(&inArtifactName, "artifactName", getFunction); nil != err {
			return nil, nil, errors.New("mandatory parameter \"artifactName\" is not set")
		}
		if err := CheckMandatoryParameter(&inArtifactKind, "artifactKind", getFunction); nil != err {
			return nil, nil, errors.New("mandatory parameter \"artifactKind\" is not set")
		}
		inArtifactDescription = getFunction("description")
		inArtifactPrefix = getFunction("prefix")
		inArtifactSuffix = getFunction("suffix")
	default:
		return nil, nil, fmt.Errorf("versionControl should be \"{auto,conf,input}\", got %q", inVersionControl)
	}

	if err := CheckArtifactKind(inArtifactKind); nil != err {
		return nil, nil, err
	}

	if inAPIVersion != currentAPIVersion {
		return nil, nil, fmt.Errorf("apiVersion should be %q, got %q", currentAPIVersion, inAPIVersion)
	}

	return &artifactConfigs{
			endpoint:              CreateArtifactEndpoint(gVsystemInfo.Endpoint, inAPIVersion, "artifacts"),
			inAPIVersion:          inAPIVersion,
			inOperatorName:        GetString("processName"),
			inArtifactName:        inArtifactName,
			inArtifactKind:        inArtifactKind,
			inArtifactDescription: inArtifactDescription,
			inArtifactPrefix:      inArtifactPrefix,
			inArtifactSuffix:      inArtifactSuffix,
		}, &operatorConfigs{
			service: inService,
		}, nil
}

func createURIFromConfig(inArtifact *artifactConfigs, executionID string, counterSuffix uint64) string {
	var uniqueIdentifier = fmt.Sprintf("%v%v_%v%v", inArtifact.inArtifactPrefix, inArtifact.inOperatorName, counterSuffix, inArtifact.inArtifactSuffix)
	return createArtifactURI(executionID, uniqueIdentifier)
}

func createArtifactRequestMetaData(getFunction func(string) string, executionID string, counterSuffix uint64, path string) (*ArtifactPostRequestMetaData, *artifactConfigs, *operatorConfigs, error) {

	inArtifact, operatorConfigs, err := generateArtifactConfiguration(getFunction)
	if nil != err {
		return nil, nil, nil, err
	}

	var inPath string
	switch path {
	case "":
		// createNewArtifactRequestMetaData
		inPath = createURIFromConfig(inArtifact, executionID, counterSuffix)
	default:
		// createExistingArtifactRequestMetaData
		inPath = path
	}

	return &ArtifactPostRequestMetaData{
		Name:        inArtifact.inArtifactName,
		Kind:        inArtifact.inArtifactKind,
		URI:         inPath,
		Description: inArtifact.inArtifactDescription,
		Type:        "EXECUTION",
		ExecutionID: executionID,
	}, inArtifact, operatorConfigs, nil
}

func createArtifactURI(executionID string, uniqueIdentifier string) string {
	const fixedPath = "shared/ml/artifacts/executions"
	return fmt.Sprintf("%v/%v/%v", fixedPath, executionID, uniqueIdentifier)
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
	OperatorName: "ArtifactProducer",
	Operation:    "artifact production",
	OperatorPath: "com.sap.ml.artifact.producer",
}

type operatorConfigs struct {
	service string
}

type Connection struct {
	internalCon string
	externalCon string
}

type artifactConfigs struct {
	endpoint              string
	inAPIVersion          string
	inOperatorName        string
	inArtifactName        string
	inArtifactKind        string
	inArtifactDescription string
	inArtifactPrefix      string
	inArtifactSuffix      string
}

type requestSDLInfos struct {
	httpMethod         string
	endpoint           string
	bearerToken        string
	payload            io.Reader
	contentType        string
	expectedStatusCode int
}
